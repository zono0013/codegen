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
	"regexp"
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
	IsTimestamp      bool // google.protobuf.Timestamp（time.Time）
	IsHidden         bool // API に出さない（DTO から外す）
	IsPaging         bool
	IsList           bool
	IsDecimal        bool
	DecimalPrecision int
	DecimalScale     int
}

// hasKeyMarker は @decimal / repeated と併用できないアノテーションを持つか。
func (f field) hasKeyMarker() bool {
	return f.IsPK || f.IsUnique || f.IsEmail || f.IsRequired || f.IsPaging
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
}

type inputField struct {
	GoName        string // initialism 正規化済み（例 "ID"）
	JsonName      string // proto のフィールド名。JSON キー / URL param 名
	ParamName     string
	GoType        string // usecase package から見た型
	HandlerGoType string // handler package から見た型（非 entity message に "usecase." が付く）
	// ScalarGoType は path / query の文字列から読める scalar の Go 型（string / bool / int32 / int64 / uint32 / uint64）。
	// 読めないフィールドは ""
	ScalarGoType string
	// ParseFormat は文字列を ScalarGoType に変換する strconv の呼び出し（%s に文字列の式が入る）。string は ""
	ParseFormat string
	// ParsedConv は ParseFormat の結果（int64 / uint64）を ScalarGoType に変換する型名。変換が要らなければ ""
	ParsedConv string
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
	ServiceName      string
	HandlerTypeName  string
	UsecaseTypeName  string
	UsecaseParamName string
	Methods          []method
	AnyReturnsEntity bool
	NeedsDTO         bool // usecase の型が dto を参照するか（entity を返す rpc、Output や非 entity message の中の entity）
	NonEntityTypes   []outputType
	NeedsStrconv     bool
	NeedsTime        bool // usecase の型が time.Time（google.protobuf.Timestamp）を使うか
	HandlerNeedsTime bool // Handler の body の型や path / query の読み出しが time を使うか
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
				// entity の中の入れ子の message は entity ではない
				for _, nested := range msg.Messages {
					if err := checkEntityOnlyMarkers(nested); err != nil {
						return nil, err
					}
				}
				continue
			}
			if err := checkEntityOnlyMarkers(msg); err != nil {
				return nil, err
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
			for _, m := range svc.Methods {
				// rpc の request / response に entity そのものを使うと、request では主キーや @hidden のフィールドまで外から書け、
				// response では @hidden のフィールドが出る（DTO を通らない）。rpc ごとの message で包む
				for _, side := range []struct {
					msg  *protogen.Message
					name string
				}{{m.Input, "request"}, {m.Output, "response"}} {
					if e, ok := entityByMsg[side.msg.Desc.FullName()]; ok {
						return nil, fmt.Errorf("rpc %s: @entity %s is used directly as the %s (define a %s%s message)", m.GoName, e.Name, side.name, m.GoName, strings.ToUpper(side.name[:1])+side.name[1:])
					}
				}
				if path, name := entityInMessage(m.Input, entityByMsg, map[*protogen.Message]bool{}); name != "" {
					return nil, fmt.Errorf("rpc %s: request %s contains @entity %s at %q (write the request as scalar fields)", m.GoName, m.Input.Desc.Name(), name, path)
				}
			}
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
		Plural:      naming.Plural(lower),
		SnakePlural: naming.Plural(snake),
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
			SnakeName:        string(f.Desc.Name()),
			ProtoType:        protoType,
			IsPK:             annotation.HasMarker(comment, "@pk"),
			IsUnique:         annotation.HasMarker(comment, "@unique"),
			IsEmail:          annotation.HasMarker(comment, "@email"),
			IsRequired:       annotation.HasMarker(comment, "@required"),
			IsTimestamp:      rest.IsTimestamp(f),
			IsHidden:         rest.IsHidden(f),
			IsPaging:         isPaging,
			IsList:           f.Desc.IsList(),
			IsDecimal:        isDecimal,
			DecimalPrecision: precision,
			DecimalScale:     scale,
		}
		if err := checkTimestampMarker(name, f); err != nil {
			return nil, err
		}
		// time.Time を引数に取る SelectBy / SelectByCursor は生成していない（repository が time を import しない）。
		// timestamptz の配列の列にも対応していない
		if spec.IsTimestamp && (spec.IsPK || spec.IsUnique || isPaging || spec.IsList) {
			return nil, fmt.Errorf("message %s: field %q: google.protobuf.Timestamp cannot be combined with @pk/@unique/@paging or `repeated` in @entity", name, f.GoName)
		}
		// 列名・DTO の JSON のキーは proto のフィールド名をそのまま使う（TS・OpenAPI と同じ）。Go の名前から作り直すと、
		// 頭字語や camelCase の名前で TS とずれる
		if !lowerSnake.MatchString(string(f.Desc.Name())) {
			return nil, fmt.Errorf("message %s: field %q: @entity field names must be lower_snake_case", name, f.Desc.Name())
		}
		if spec.IsDecimal {
			if spec.ProtoType != "string" {
				return nil, fmt.Errorf("message %s: field %q: @decimal requires proto type \"string\" but got %q (precision loss prevention)", name, f.GoName, spec.ProtoType)
			}
			if spec.hasKeyMarker() {
				return nil, fmt.Errorf("message %s: field %q: @decimal cannot be combined with @pk/@unique/@email/@required/@paging", name, f.GoName)
			}
		}
		// 主キー・cursor の基準を出さないと、クライアントは取得や次のページに渡す値を持てない
		if spec.IsHidden && (spec.IsPK || isPaging) {
			return nil, fmt.Errorf("message %s: field %q: @hidden cannot be combined with @pk/@paging", name, f.GoName)
		}
		// 検査は文字列を対象にしたもの（trim / 空 / "@" を含むか）なので、ほかの型では生成したコードがコンパイルできない
		if (spec.IsEmail || spec.IsRequired) && spec.ProtoType != "string" {
			return nil, fmt.Errorf("message %s: field %q: @email/@required requires proto type \"string\" but got %q", name, f.GoName, spec.ProtoType)
		}
		if spec.IsList && spec.hasKeyMarker() {
			return nil, fmt.Errorf("message %s: field %q: `repeated` cannot be combined with @pk/@unique/@email/@required/@paging in @entity", name, f.GoName)
		}

		listPrefix := ""
		if spec.IsList {
			listPrefix = "[]"
		}
		switch {
		case spec.IsTimestamp:
			spec.GoType = "time.Time"
		case spec.IsDecimal:
			spec.GoType = listPrefix + "decimal.Decimal"
		default:
			goType, ok := entityScalarGoType(f.Desc.Kind())
			if !ok {
				return nil, fmt.Errorf("message %s: field %q: %s is not supported in @entity (use a scalar type)", name, f.GoName, describeType(f))
			}
			spec.GoType = listPrefix + goType
		}

		e.Fields = append(e.Fields, spec)
		e.HasDecimal = e.HasDecimal || spec.IsDecimal
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
	usesDTO  bool // Output や非 entity message（Input から辿るものも含む）が dto の型を参照したか
	usesTime bool // time.Time（google.protobuf.Timestamp）を参照したか
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
				ScalarGoType:  scalarGoType(f),
			}
			in.ParseFormat, in.ParsedConv = scalarParse(in.ScalarGoType)
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
				in := inputs[f]
				mt.PathParamFields = append(mt.PathParamFields, in)
				s.addParamImports(in)
			}
			for _, f := range fields.Body {
				mt.BodyFields = append(mt.BodyFields, inputs[f])
				s.HandlerNeedsTime = s.HandlerNeedsTime || strings.Contains(inputs[f].HandlerGoType, "time.Time")
			}
			for _, f := range fields.Query {
				in := inputs[f]
				mt.QueryFields = append(mt.QueryFields, in)
				s.addParamImports(in)
			}
		}

		s.Methods = append(s.Methods, mt)
	}

	s.NeedsDTO = s.AnyReturnsEntity || r.usesDTO
	s.NeedsTime = r.usesTime
	s.NonEntityTypes = slices.SortedFunc(maps.Values(r.types), func(a, b outputType) int { return strings.Compare(a.Name, b.Name) })
	return s
}

// addParamImports は path / query の値を読むのに Handler が使う package を記録する。
func (s *service) addParamImports(in inputField) {
	s.NeedsStrconv = s.NeedsStrconv || strings.HasPrefix(in.ParseFormat, "strconv.")
	s.HandlerNeedsTime = s.HandlerNeedsTime || in.ScalarGoType == "time.Time"
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
		GoType:  r.outputGoType(f),
		JsonTag: string(f.Desc.Name()),
	}
}

// outputGoType は Output と非 entity message（Input から辿るものも含む）のフィールド型。entity は DTO にする。
// entity は json タグを持たないので、そのまま JSON にするとキーが Go のフィールド名になり、OpenAPI・TS と食い違う。
func (r *typeResolver) outputGoType(f *protogen.Field) string {
	if f.Desc.IsMap() {
		return "map[" + r.goType(f.Message.Fields[0], "") + "]" + r.outputGoType(f.Message.Fields[1])
	}
	if f.Desc.Kind() == protoreflect.MessageKind && f.Message != nil {
		if e, ok := r.entities[f.Message.Desc.FullName()]; ok {
			r.usesDTO = true
			if f.Desc.IsList() {
				return "[]*dto." + e.Name + "DTO"
			}
			return "*dto." + e.Name + "DTO"
		}
	}
	return r.goType(f, "")
}

// goType は Input / Output struct のフィールド型。非 entity message は usecase package に生成する struct を指し、
// usecase の外（handler）から参照するときは pkg に "usecase." を渡す。
func (r *typeResolver) goType(f *protogen.Field, pkg string) string {
	if f.Desc.IsMap() {
		return "map[" + r.goType(f.Message.Fields[0], pkg) + "]" + r.goType(f.Message.Fields[1], pkg)
	}
	if rest.IsTimestamp(f) {
		r.usesTime = true
		if f.Desc.IsList() {
			return "[]time.Time"
		}
		return "time.Time"
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

// referencedMessage は f が指す message（map なら value 側）。scalar と Timestamp（time.Time）なら nil。
func referencedMessage(f *protogen.Field) *protogen.Message {
	if f.Desc.IsMap() {
		f = f.Message.Fields[1]
	}
	if f.Desc.Kind() != protoreflect.MessageKind || rest.IsTimestamp(f) {
		return nil
	}
	return f.Message
}

// scalarGoType は Handler が path / query の文字列から読むときの型。読めないフィールドは ""。
// goTypeFromKind と同じ型にする（Input のフィールドにそのまま代入するため）。
func scalarGoType(f *protogen.Field) string {
	if !rest.IsQueryable(f) {
		return ""
	}
	if rest.IsTimestamp(f) {
		return "time.Time"
	}
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		return "string"
	case protoreflect.BoolKind:
		return "bool"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64"
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64"
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32"
	default:
		return "int32"
	}
}

// scalarParse は scalarGoType の型へ文字列から変換する strconv の呼び出しと、その結果に掛ける型変換を返す。
func scalarParse(goType string) (format, conv string) {
	switch goType {
	case "bool":
		return "strconv.ParseBool(%s)", ""
	case "int32":
		return "strconv.ParseInt(%s, 10, 32)", "int32"
	case "int64":
		return "strconv.ParseInt(%s, 10, 64)", ""
	case "uint32":
		return "strconv.ParseUint(%s, 10, 32)", "uint32"
	case "uint64":
		return "strconv.ParseUint(%s, 10, 64)", ""
	case "time.Time":
		return "time.Parse(time.RFC3339, %s)", ""
	}
	return "", ""
}

// entityInMessage は msg（とそこから辿る message、map の値）が持つ entity のフィールドを探し、
// 見つけたらフィールドのパス（"wrap.item"）と entity の名前を返す。request に entity を置くと、
// 主キーや @hidden のフィールドまで外から書けてしまう（一括代入）ので、生成のときに止める。
func entityInMessage(msg *protogen.Message, entities map[protoreflect.FullName]*entity, seen map[*protogen.Message]bool) (path, name string) {
	if seen[msg] {
		return "", ""
	}
	seen[msg] = true
	for _, f := range msg.Fields {
		ref := referencedMessage(f)
		if ref == nil {
			continue
		}
		if e, ok := entities[ref.Desc.FullName()]; ok {
			return string(f.Desc.Name()), e.Name
		}
		if p, n := entityInMessage(ref, entities, seen); n != "" {
			return string(f.Desc.Name()) + "." + p, n
		}
	}
	return "", ""
}

// entityOnlyMarkers は entity のフィールドでだけ意味を持つアノテーション。@email は TS・OpenAPI がどの message でも使う。
var entityOnlyMarkers = []string{"@pk", "@unique", "@paging", "@required", "@decimal", "@hidden"}

// checkEntityOnlyMarkers は entity ではない message（入れ子も）に entity 用のアノテーションが付いていればエラーにする。
// 効かないので、付けた意図（例: @hidden で出さない）と違う出力になる。
func checkEntityOnlyMarkers(msg *protogen.Message) error {
	for _, f := range msg.Fields {
		if err := checkTimestampMarker(string(msg.Desc.Name()), f); err != nil {
			return err
		}
		comment := string(f.Comments.Leading)
		for _, m := range entityOnlyMarkers {
			if annotation.HasMarker(comment, m) || m == "@decimal" && strings.Contains(comment, "@decimal(") {
				return fmt.Errorf("message %s: field %q: %s works only on @entity fields", msg.Desc.Name(), f.GoName, m)
			}
		}
	}
	for _, nested := range msg.Messages {
		if err := checkEntityOnlyMarkers(nested); err != nil {
			return err
		}
	}
	return nil
}

// checkTimestampMarker は @timestamp が付いていればエラーにする。日時は型で表すので、注釈は効かない（int64 のままになる）。
func checkTimestampMarker(msg string, f *protogen.Field) error {
	if annotation.HasMarker(string(f.Comments.Leading), "@timestamp") {
		return fmt.Errorf("message %s: field %q: @timestamp is not supported; remove it and declare date-time fields as google.protobuf.Timestamp", msg, f.Desc.Name())
	}
	return nil
}

// describeType はエラーメッセージ用の型の説明（"map<string, Meta>"、"google.protobuf.Timestamp (message)" など）。
func describeType(f *protogen.Field) string {
	switch {
	case f.Desc.IsMap():
		return fmt.Sprintf("map<%s, %s>", describeType(f.Message.Fields[0]), describeType(f.Message.Fields[1]))
	case f.Message != nil:
		return fmt.Sprintf("%q (message)", f.Message.Desc.FullName())
	case f.Enum != nil:
		return fmt.Sprintf("%q (enum)", f.Enum.Desc.FullName())
	}
	return f.Desc.Kind().String()
}

var lowerSnake = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// entityScalarGoType は Entity のフィールドの Go 型。Input / Output（goTypeFromKind）と同じ型にする。
// enum・message は DB の列にどう載せるかが決まっていないので対応しない。
func entityScalarGoType(k protoreflect.Kind) (string, bool) {
	switch k {
	case protoreflect.StringKind:
		return "string", true
	case protoreflect.BoolKind:
		return "bool", true
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return "int32", true
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return "int64", true
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return "uint32", true
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return "uint64", true
	case protoreflect.FloatKind:
		return "float32", true
	case protoreflect.DoubleKind:
		return "float64", true
	case protoreflect.BytesKind:
		return "[]byte", true
	}
	return "", false
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

// initialisms は Go の慣習で大文字で書く頭字語（golint / staticcheck の ST1003 と同じ一覧）。
var initialisms = map[string]bool{
	"ACL": true, "API": true, "ASCII": true, "CPU": true, "CSS": true, "DNS": true, "EOF": true, "GUID": true,
	"HTML": true, "HTTP": true, "HTTPS": true, "ID": true, "IP": true, "JSON": true, "LHS": true, "QPS": true,
	"RAM": true, "RHS": true, "RPC": true, "SLA": true, "SMTP": true, "SQL": true, "SSH": true, "TCP": true,
	"TLS": true, "TTL": true, "UDP": true, "UI": true, "UID": true, "UUID": true, "URI": true, "URL": true,
	"UTF8": true, "VM": true, "XML": true, "XMPP": true, "XSRF": true, "XSS": true,
}

// normalizeInitialisms は頭字語の単語を Go の慣習どおり大文字にする（"AuthorId" → "AuthorID"、"CommunityUuid" → "CommunityUUID"）。
// 単語は大文字で始まり小文字・数字が続くまとまり（protogen の GoName は snake_case の各語を先頭だけ大文字にしたもの）。
func normalizeInitialisms(s string) string {
	var b strings.Builder
	for start := 0; start < len(s); {
		end := start + 1
		for end < len(s) && !(s[end] >= 'A' && s[end] <= 'Z') {
			end++
		}
		word := s[start:end]
		if upper := strings.ToUpper(word); initialisms[upper] && word != upper {
			word = upper
		}
		b.WriteString(word)
		start = end
	}
	return b.String()
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
