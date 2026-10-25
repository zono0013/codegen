// Package gogen は server/ 配下の Go コード（Entity / Repository / Mock / Postgres 実装 / DTO /
// Usecase interface / REST Handler / DI 配線）を生成する。仕様は docs/spec.md。
//
// テンプレートは generator/<kind>/output/<出力先をミラーしたパス>/*.tpl に置き、<kind> 名で参照する。
package gogen

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"go/token"
	"io/fs"
	"maps"
	"net/http"
	"path"
	"slices"
	"strings"
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/zono0013/codegen/internal/annotation"
	"github.com/zono0013/codegen/internal/naming"
	"github.com/zono0013/codegen/internal/rest"
)

//go:embed generator runtime
var templatesFS embed.FS

// importPathPlaceholder はランタイム（runtime/**/*.go.src）の中で、生成先の import path に置き換える文字列。
// ランタイムは Go の複合リテラル（{{...}}）を含むので text/template では扱わない。
const importPathPlaceholder = "__IMPORT_PATH__"

type kind struct {
	name string
	out  func(snake string) string
}

var entityKinds = []kind{
	{"entity", func(s string) string { return "internal/domain/entity/" + s + ".gen.go" }},
	{"repository", func(s string) string { return "internal/domain/repository/" + s + "_repository.gen.go" }},
	{"mock", func(s string) string { return "internal/domain/repository/mock/mock_" + s + "_repository.gen.go" }},
	{"infra_postgres_repository", func(s string) string { return "internal/infra/repository/" + s + "_postgres_repository.gen.go" }},
	{"dto", func(s string) string { return "internal/dto/" + s + ".gen.go" }},
}

var serviceKinds = []kind{
	{"usecase", func(s string) string { return "internal/usecase/" + s + "_usecase_interface.gen.go" }},
	{"handler_rest", func(s string) string { return "internal/handler/" + s + "_handler.gen.go" }},
}

type field struct {
	GoName           string
	ParamName        string
	SnakeName        string
	ProtoType        string
	GoType           string
	IsPK             bool
	IsUnique         bool
	IsEmail          bool
	IsRequired       bool
	IsTimestamp      bool
	IsPaging         bool
	IsList           bool
	IsDecimal        bool
	DecimalPrecision int
	DecimalScale     int
}

// hasKeyMarker は @decimal / repeated と併用できないアノテーションを持つか。
func (f field) hasKeyMarker() bool {
	return f.IsPK || f.IsUnique || f.IsEmail || f.IsRequired || f.IsTimestamp || f.IsPaging
}

type entity struct {
	Name              string
	SnakeName         string
	LowerFirst        string
	Plural            string
	SnakePlural       string
	Fields            []field
	PKField           field
	UniqueFieldsNonPK []field
	PagingField       *field
	HasDecimal        bool
	// NeedsStrings / NeedsErrors は New<Entity> の検査が strings / errors を使うか（使わない import はコンパイルできない）
	NeedsStrings bool
	NeedsErrors  bool
}

type inputField struct {
	GoName        string // initialism 正規化済み（例 "ID"）
	JsonName      string // proto のフィールド名。JSON キー / URL param 名
	ParamName     string
	GoType        string // usecase package から見た型
	HandlerGoType string // handler package から見た型（非 entity message に "usecase." が付く）
	QueryGoType   string // query string から読む scalar 型。読めないフィールドは ""
}

type method struct {
	Name             string
	InputTypeName    string
	InputFields      []inputField
	PathParamFields  []inputField
	BodyFields       []inputField
	QueryFields      []inputField
	EntityGoName     string
	EntityLowerFirst string
	ReturnsEntity    bool
	ReturnsList      bool
	ReturnsOutput    bool
	OutputTypeName   string
	OutputFields     []outputField
	Http             *annotation.HTTP
	Status           string // 成功時の http.Status* 定数
}

type outputField struct {
	GoName  string
	GoType  string
	JsonTag string
}

// outputType は Output struct から参照される非 entity message の Go 型定義。
type outputType struct {
	Name   string
	Fields []outputField
}

type service struct {
	ServiceName       string
	HandlerTypeName   string
	UsecaseTypeName   string
	UsecaseParamName  string
	Methods           []method
	AnyReturnsEntity  bool
	AnyUsesEntity     bool
	HandlerUsesEntity bool // Output の entity は usecase 側にしか出ないので AnyUsesEntity と別に持つ
	NonEntityTypes    []outputType
	NeedsStrconv      bool
}

var statusConst = map[int]string{
	http.StatusOK:        "http.StatusOK",
	http.StatusCreated:   "http.StatusCreated",
	http.StatusNoContent: "http.StatusNoContent",
}

// Params は生成先のプロジェクトごとに変わる値。
type Params struct {
	// ImportPath は生成先ディレクトリ（server の出力先）の Go の import path。生成物どうしの import に使う。
	ImportPath string
}

// New は params に従い、server の出力先からの相対パス → Go コードを返すジェネレータを作る。
func New(params Params) func([]*protogen.File) (map[string][]byte, error) {
	return func(files []*protogen.File) (map[string][]byte, error) { return generate(files, params) }
}

func generate(files []*protogen.File, params Params) (map[string][]byte, error) {
	if params.ImportPath == "" {
		return nil, errors.New("gogen: ImportPath が空")
	}
	tmpls, err := loadTemplates(params)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	render := func(kind, path string, data any) error {
		var buf bytes.Buffer
		if err := tmpls[kind].Execute(&buf, data); err != nil {
			return fmt.Errorf("%s: %w", kind, err)
		}
		out[path] = buf.Bytes()
		return nil
	}

	// service の解析で entity を参照するので、先に全ファイルの entity を集める
	var entities []*entity
	entityByMsg := map[protoreflect.FullName]*entity{}
	for _, f := range files {
		for _, msg := range f.Messages {
			e, err := parseEntity(msg)
			if err != nil {
				return nil, err
			}
			if e != nil {
				entities = append(entities, e)
				entityByMsg[msg.Desc.FullName()] = e
			}
		}
	}
	for _, e := range entities {
		for _, k := range entityKinds {
			if err := render(k.name, k.out(e.SnakeName), e); err != nil {
				return nil, err
			}
		}
	}

	var services []service
	for _, f := range files {
		for _, svc := range f.Services {
			s := buildService(svc, entityByMsg)
			services = append(services, s)
			snake := strings.TrimSuffix(naming.ToSnake(svc.GoName), "_service")
			for _, k := range serviceKinds {
				if err := render(k.name, k.out(snake), s); err != nil {
					return nil, err
				}
			}
		}
	}

	if len(services) > 0 {
		slices.SortFunc(services, func(a, b service) int { return strings.Compare(a.ServiceName, b.ServiceName) })
		if err := render("di", "internal/di/handlers.gen.go", struct{ Services []service }{services}); err != nil {
			return nil, err
		}
	}
	if err := addRuntime(out, params.ImportPath); err != nil {
		return nil, err
	}
	// migration-gen（ランタイム）が entity.All を参照するので、entity が無くても出す
	sorted := slices.SortedFunc(slices.Values(entities), func(a, b *entity) int { return strings.Compare(a.Name, b.Name) })
	if err := render("entity_registry", "internal/domain/entity/registry.gen.go", struct{ Entities []*entity }{sorted}); err != nil {
		return nil, err
	}
	return out, nil
}

// addRuntime は生成コードが依存するランタイム（errs / tx / Postgres のエラー判定 / migration-gen）を out に加える。
// proto の内容に関係なく毎回出す。runtime/<出力先をミラーしたパス>/<name>.go.src → <name>.gen.go。
func addRuntime(out map[string][]byte, importPath string) error {
	return fs.WalkDir(templatesFS, "runtime", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go.src") {
			return err
		}
		raw, err := templatesFS.ReadFile(p)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(strings.TrimPrefix(p, "runtime/"), ".go.src") + ".gen.go"
		// 先頭に空行を入れ、生成ヘッダーがパッケージの doc コメントに混ざらないようにする
		out[name] = append([]byte("\n"), bytes.ReplaceAll(raw, []byte(importPathPlaceholder), []byte(importPath))...)
		return nil
	})
}

func loadTemplates(params Params) (map[string]*template.Template, error) {
	funcs := template.FuncMap{
		"gormTag":    gormTag,
		"importPath": func() string { return params.ImportPath },
	}
	out := map[string]*template.Template{}
	err := fs.WalkDir(templatesFS, "generator", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(p) != ".tpl" {
			return err
		}
		name := strings.Split(p, "/")[1]
		raw, err := templatesFS.ReadFile(p)
		if err != nil {
			return err
		}
		t, err := template.New(name).Funcs(funcs).Parse(string(raw))
		if err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		out[name] = t
		return nil
	})
	return out, err
}

func parseEntity(msg *protogen.Message) (*entity, error) {
	if !rest.IsEntity(msg) {
		return nil, nil
	}
	name := msg.GoIdent.GoName
	snake := naming.ToSnake(name)
	lower := lowerFirst(name)
	e := &entity{
		Name:        name,
		SnakeName:   snake,
		LowerFirst:  lower,
		Plural:      lower + "s",
		SnakePlural: snake + "s",
	}

	for _, f := range msg.Fields {
		comment := string(f.Comments.Leading)
		protoType := f.Desc.Kind().String()

		isPaging := annotation.HasMarker(comment, "@paging")
		if isPaging {
			if protoType != "int64" && protoType != "int32" && protoType != "string" {
				return nil, fmt.Errorf("message %s: field %q: @paging field has unsupported proto type %q (allowed: int64, int32, string)", name, f.GoName, protoType)
			}
			if e.PagingField != nil {
				return nil, fmt.Errorf("message %s: only one @paging field is allowed, found multiple", name)
			}
		}

		precision, scale, isDecimal, err := annotation.ParseDecimal(comment)
		if err != nil {
			return nil, fmt.Errorf("message %s: field %q: %w", name, f.GoName, err)
		}

		spec := field{
			GoName:           normalizeInitialisms(f.GoName),
			ParamName:        lowerFirst(f.GoName),
			SnakeName:        naming.ToSnake(f.GoName),
			ProtoType:        protoType,
			IsPK:             annotation.HasMarker(comment, "@pk"),
			IsUnique:         annotation.HasMarker(comment, "@unique"),
			IsEmail:          annotation.HasMarker(comment, "@email"),
			IsRequired:       annotation.HasMarker(comment, "@required"),
			IsTimestamp:      annotation.HasMarker(comment, "@timestamp"),
			IsPaging:         isPaging,
			IsList:           f.Desc.IsList(),
			IsDecimal:        isDecimal,
			DecimalPrecision: precision,
			DecimalScale:     scale,
		}
		if spec.IsDecimal {
			if spec.ProtoType != "string" {
				return nil, fmt.Errorf("message %s: field %q: @decimal requires proto type \"string\" but got %q (precision loss prevention)", name, f.GoName, spec.ProtoType)
			}
			if spec.hasKeyMarker() {
				return nil, fmt.Errorf("message %s: field %q: @decimal cannot be combined with @pk/@unique/@email/@required/@timestamp/@paging", name, f.GoName)
			}
		}
		if spec.IsList && spec.hasKeyMarker() {
			return nil, fmt.Errorf("message %s: field %q: `repeated` cannot be combined with @pk/@unique/@email/@required/@timestamp/@paging in @entity", name, f.GoName)
		}

		listPrefix := ""
		if spec.IsList {
			listPrefix = "[]"
		}
		switch {
		case spec.IsTimestamp:
			spec.GoType = "time.Time"
			base := strings.TrimSuffix(spec.GoName, "Unix")
			spec.GoName = base
			spec.ParamName = lowerFirst(base)
			spec.SnakeName = naming.ToSnake(base)
		case spec.IsDecimal:
			spec.GoType = listPrefix + "decimal.Decimal"
		case slices.Contains([]string{"string", "int32", "int64", "bool"}, spec.ProtoType):
			spec.GoType = listPrefix + spec.ProtoType
		default:
			spec.GoType = listPrefix + "string"
		}

		e.Fields = append(e.Fields, spec)
		e.HasDecimal = e.HasDecimal || spec.IsDecimal
		// entity.gen.go.tpl の New<Entity> の検査と同じ条件
		e.NeedsStrings = e.NeedsStrings || spec.IsRequired && spec.GoType == "string" || spec.IsEmail
		e.NeedsErrors = e.NeedsErrors || e.NeedsStrings || spec.IsPK && spec.GoType == "string"
		if spec.IsPK {
			e.PKField = spec
		}
		if spec.IsUnique && !spec.IsPK {
			e.UniqueFieldsNonPK = append(e.UniqueFieldsNonPK, spec)
		}
		if isPaging {
			e.PagingField = &spec
		}
	}

	if p := e.PagingField; p != nil && !p.IsPK && !p.IsUnique {
		return nil, fmt.Errorf("message %s: @paging field %q must also have @pk or @unique", name, p.GoName)
	}
	return e, nil
}

func gormTag(f field) string {
	tags := []string{"column:" + f.SnakeName}
	switch {
	case f.IsList:
		tags = append(tags, "type:jsonb", "serializer:json")
	case f.IsDecimal:
		tags = append(tags, fmt.Sprintf("type:numeric(%d,%d)", f.DecimalPrecision, f.DecimalScale))
	}
	if f.IsPK {
		tags = append(tags, "primaryKey")
	}
	if f.IsUnique && !f.IsPK {
		tags = append(tags, "uniqueIndex")
	}
	if !f.IsPK {
		tags = append(tags, "not null")
	}
	return strings.Join(tags, ";")
}

type typeResolver struct {
	entities map[protoreflect.FullName]*entity
	names    map[protoreflect.FullName]string
	used     map[string]bool
	types    map[string]outputType
}

func buildService(svc *protogen.Service, entities map[protoreflect.FullName]*entity) service {
	usecaseType := strings.TrimSuffix(svc.GoName, "Service") + "Usecase"
	s := service{
		ServiceName:      svc.GoName,
		HandlerTypeName:  strings.TrimSuffix(svc.GoName, "Service") + "Handler",
		UsecaseTypeName:  usecaseType,
		UsecaseParamName: lowerFirst(usecaseType),
	}

	// 非 entity message は usecase package に struct として出すので、rpc 由来の型名と衝突させない
	r := &typeResolver{entities: entities, names: map[protoreflect.FullName]string{}, used: map[string]bool{}, types: map[string]outputType{}}
	for _, m := range svc.Methods {
		r.used[m.GoName+"Input"] = true
		r.used[m.GoName+"Output"] = true
	}

	for _, m := range svc.Methods {
		mt := method{
			Name:          m.GoName,
			InputTypeName: m.GoName + "Input",
			Http:          annotation.ParseHTTP(string(m.Comments.Leading)),
		}

		inputs := map[*protogen.Field]inputField{}
		for _, f := range m.Input.Fields {
			goType := r.goType(f, "")
			in := inputField{
				GoName:        normalizeInitialisms(f.GoName),
				JsonName:      string(f.Desc.Name()),
				ParamName:     lowerFirst(f.GoName),
				GoType:        goType,
				HandlerGoType: r.goType(f, "usecase."),
				QueryGoType:   queryScalarGoType(f),
			}
			if strings.Contains(goType, "entity.") {
				s.AnyUsesEntity = true
				s.HandlerUsesEntity = true
			}
			r.collect(f)
			inputs[f] = in
			mt.InputFields = append(mt.InputFields, in)
		}

		shape := rest.ResponseShape(m.Output)
		ent := entities[entityName(shape)]
		switch {
		case shape.Kind == rest.ShapeEntity && ent != nil:
			mt.ReturnsEntity = true
		case shape.Kind == rest.ShapeList && ent != nil:
			mt.ReturnsList = true
		case shape.Kind == rest.ShapeEmpty:
		default:
			shape.Kind = rest.ShapeOutput
			mt.ReturnsOutput = true
			mt.OutputTypeName = m.GoName + "Output"
			for _, f := range m.Output.Fields {
				mt.OutputFields = append(mt.OutputFields, r.outputField(f))
			}
			for _, f := range m.Output.Fields {
				r.collect(f)
			}
			if r.usesEntity(m.Output) {
				s.AnyUsesEntity = true
			}
		}
		if mt.ReturnsEntity || mt.ReturnsList {
			// 戻り値は DTO に変換済みなので entity package は import しない
			mt.EntityGoName = ent.Name
			mt.EntityLowerFirst = lowerFirst(ent.Name)
			s.AnyReturnsEntity = true
		}

		if mt.Http != nil {
			mt.Status = statusConst[rest.SuccessStatus(mt.Http.Method, shape)]
			fields := rest.Partition(m.Input, mt.Http)
			for _, f := range fields.Path {
				mt.PathParamFields = append(mt.PathParamFields, inputs[f])
			}
			for _, f := range fields.Body {
				mt.BodyFields = append(mt.BodyFields, inputs[f])
			}
			for _, f := range fields.Query {
				in := inputs[f]
				mt.QueryFields = append(mt.QueryFields, in)
				s.NeedsStrconv = s.NeedsStrconv || in.QueryGoType == "int32" || in.QueryGoType == "int64"
			}
		}

		s.Methods = append(s.Methods, mt)
	}

	s.NonEntityTypes = slices.SortedFunc(maps.Values(r.types), func(a, b outputType) int { return strings.Compare(a.Name, b.Name) })
	return s
}

func entityName(s rest.Shape) protoreflect.FullName {
	if s.Entity == nil {
		return ""
	}
	return s.Entity.Desc.FullName()
}

// name は非 entity message の usecase package 内での型名。衝突したら末尾に "_" を足してずらす。
func (r *typeResolver) name(msg *protogen.Message) string {
	if n, ok := r.names[msg.Desc.FullName()]; ok {
		return n
	}
	n := msg.GoIdent.GoName
	for r.used[n] {
		n += "_"
	}
	r.names[msg.Desc.FullName()] = n
	r.used[n] = true
	return n
}

func (r *typeResolver) outputField(f *protogen.Field) outputField {
	return outputField{
		GoName:  normalizeInitialisms(f.GoName),
		GoType:  r.goType(f, ""),
		JsonTag: string(f.Desc.Name()),
	}
}

// goType は Input / Output struct のフィールド型。非 entity message は usecase package に生成する struct を指し、
// usecase の外（handler）から参照するときは pkg に "usecase." を渡す。
func (r *typeResolver) goType(f *protogen.Field, pkg string) string {
	if f.Desc.IsMap() {
		return "map[" + r.goType(f.Message.Fields[0], pkg) + "]" + r.goType(f.Message.Fields[1], pkg)
	}
	if f.Desc.Kind() == protoreflect.MessageKind && f.Message != nil {
		if _, ok := r.entities[f.Message.Desc.FullName()]; !ok {
			if f.Desc.IsList() {
				return "[]*" + pkg + r.name(f.Message)
			}
			return "*" + pkg + r.name(f.Message)
		}
	}
	return goTypeFromKind(f, r.entities)
}

// collect は f が参照する非 entity message を、子のフィールドも辿って Go 型定義として登録する。
func (r *typeResolver) collect(f *protogen.Field) {
	msg := referencedMessage(f)
	if msg == nil {
		return
	}
	if _, ok := r.entities[msg.Desc.FullName()]; ok {
		return
	}
	name := r.name(msg)
	if _, seen := r.types[name]; seen {
		return
	}
	t := outputType{Name: name}
	for _, child := range msg.Fields {
		t.Fields = append(t.Fields, r.outputField(child))
	}
	r.types[name] = t
	for _, child := range msg.Fields {
		r.collect(child)
	}
}

func (r *typeResolver) usesEntity(msg *protogen.Message) bool {
	for _, f := range msg.Fields {
		ref := referencedMessage(f)
		if ref == nil {
			continue
		}
		if _, ok := r.entities[ref.Desc.FullName()]; ok {
			return true
		}
		if !f.Desc.IsMap() && r.usesEntity(ref) {
			return true
		}
	}
	return false
}

// referencedMessage は f が指す message（map なら value 側）。scalar なら nil。
func referencedMessage(f *protogen.Field) *protogen.Message {
	if f.Desc.IsMap() {
		f = f.Message.Fields[1]
	}
	if f.Desc.Kind() != protoreflect.MessageKind {
		return nil
	}
	return f.Message
}

// queryScalarGoType は Handler が query string から読むときの型。読めないフィールドは ""。
func queryScalarGoType(f *protogen.Field) string {
	if !rest.IsQueryable(f) {
		return ""
	}
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "int64"
	default:
		return "int32"
	}
}

func goTypeFromKind(f *protogen.Field, entities map[protoreflect.FullName]*entity) string {
	list := func(t string) string {
		if f.Desc.IsList() {
			return "[]" + t
		}
		return t
	}
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return list("string")
	case protoreflect.BoolKind:
		return list("bool")
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return list("int32")
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return list("int64")
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return list("uint32")
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return list("uint64")
	case protoreflect.FloatKind:
		return list("float32")
	case protoreflect.DoubleKind:
		return list("float64")
	case protoreflect.BytesKind:
		return list("[]byte")
	case protoreflect.MessageKind:
		if f.Message != nil {
			if e, ok := entities[f.Message.Desc.FullName()]; ok {
				return list("*entity." + e.Name)
			}
		}
		// 対応する Go 型が無い（生成しない）message は互換のため string にフォールバックする
		return "string"
	case protoreflect.EnumKind:
		if f.Enum != nil {
			return list(f.Enum.GoIdent.GoName)
		}
		return "int32"
	default:
		return "string"
	}
}

var initialisms = []struct{ from, to string }{
	{"Id", "ID"}, {"Url", "URL"}, {"Api", "API"}, {"Http", "HTTP"}, {"Json", "JSON"}, {"Xml", "XML"},
}

// normalizeInitialisms は末尾の "Id" / "Url" などを Go の慣習どおり大文字にする（"AuthorId" → "AuthorID"）。
func normalizeInitialisms(s string) string {
	for _, r := range initialisms {
		if s == r.from {
			return r.to
		}
		if strings.HasSuffix(s, r.from) && len(s) > len(r.from) {
			if prev := s[len(s)-len(r.from)-1]; prev >= 'a' && prev <= 'z' {
				s = s[:len(s)-len(r.from)] + r.to
			}
		}
	}
	return s
}

// lowerFirst は引数名・変数名用。Go の予約語になる場合は "_" を付ける（"type" → "type_"）。
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	lowered := strings.ToLower(s[:1]) + s[1:]
	if token.IsKeyword(lowered) {
		return lowered + "_"
	}
	return lowered
}
