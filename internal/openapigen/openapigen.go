// Package openapigen は docs/openapi/ 配下の OpenAPI 仕様を生成する。出力構成は docs/spec.md。
//
// proto の message をそのまま写さず、生成 REST Handler の実際の振る舞い（internal/rest）を記述する。
package openapigen

import (
	"bytes"
	"fmt"
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
	"gopkg.in/yaml.v3"

	"github.com/zono0013/codegen/internal/annotation"
	"github.com/zono0013/codegen/internal/rest"
)

// RootFile は out（docs/openapi）からのルートファイルのパス。
const RootFile = "openapi.gen.yaml"

// server/pkg/util/errs の Problem に対応する component 名。
const (
	problemSchema    = "ProblemDetails"
	fieldErrorSchema = "FieldError"
	problemResponse  = "Problem"
	problemOrigin    = "server/pkg/util/errs"
)

var methodOrder = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

type Params struct {
	Title   string
	Version string
}

// New は params を info に使い、docs/openapi/ からの相対パス → YAML を返すジェネレータを作る。
func New(params Params) func([]*protogen.File) (map[string][]byte, error) {
	return func(files []*protogen.File) (map[string][]byte, error) { return generate(files, params) }
}

type component struct {
	origin protoreflect.FullName // 同名の別 message を検出するため
	node   *yaml.Node
}

type generator struct {
	schemas map[string]*component
}

func generate(files []*protogen.File, params Params) (map[string][]byte, error) {
	g := &generator{schemas: map[string]*component{}}
	g.addProblemSchemas()

	files = slices.Clone(files)
	slices.SortFunc(files, func(a, b *protogen.File) int { return strings.Compare(a.Desc.Path(), b.Desc.Path()) })

	var tags []*yaml.Node
	var pathOrder []string
	ops := map[string]map[string]*yaml.Node{} // path → method → operation
	operationIDs := map[string]string{}
	for _, f := range files {
		for _, svc := range f.Services {
			tag := mapping("name", str(svc.GoName))
			if d := annotation.Description(string(svc.Comments.Leading)); d != "" {
				add(tag, "description", str(d))
			}
			tags = append(tags, tag)
			for _, m := range svc.Methods {
				h := annotation.ParseHTTP(string(m.Comments.Leading))
				if h == nil {
					continue // Handler が登録されない rpc
				}
				if prev, dup := operationIDs[m.GoName]; dup {
					return nil, fmt.Errorf("operationId %q が %s と %s で重複している", m.GoName, prev, svc.GoName)
				}
				operationIDs[m.GoName] = svc.GoName
				op, err := g.operation(svc, m, h)
				if err != nil {
					return nil, err
				}
				if ops[h.Path] == nil {
					ops[h.Path] = map[string]*yaml.Node{}
					pathOrder = append(pathOrder, h.Path)
				}
				ops[h.Path][h.Method] = op
			}
		}
	}

	nodes := map[string]*yaml.Node{}
	paths := mapping()
	for _, p := range pathOrder {
		item := mapping()
		for _, method := range methodOrder {
			if op, ok := ops[p][method]; ok {
				add(item, strings.ToLower(method), op)
			}
		}
		file := pathFile(p)
		if _, clash := nodes[file]; clash {
			return nil, fmt.Errorf("path %q が他の path と同じファイル名 %s になる", p, file)
		}
		nodes[file] = item
		add(paths, p, fileRef(file))
	}

	schemas := mapping()
	for _, name := range slices.Sorted(maps.Keys(g.schemas)) {
		nodes[schemaFile(name)] = g.schemas[name].node
		add(schemas, name, ref(name))
	}

	nodes[responseFile(problemResponse)] = mapping(
		"description", str("エラー（RFC 9457 Problem Details）"),
		"content", mapping("application/problem+json", mapping("schema", ref(problemSchema))),
	)
	nodes[RootFile] = mapping(
		"openapi", str("3.0.3"),
		"info", mapping("title", str(params.Title), "version", str(params.Version)),
		"tags", seq(tags...),
		"paths", paths,
		"components", mapping(
			"schemas", schemas,
			"responses", mapping(problemResponse, fileRef(responseFile(problemResponse))),
		),
	)

	out := map[string][]byte{}
	for name, node := range nodes {
		b, err := encode(node, name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out[name] = b
	}
	return out, nil
}

func (g *generator) operation(svc *protogen.Service, m *protogen.Method, h *annotation.HTTP) (*yaml.Node, error) {
	op := mapping("tags", seq(str(svc.GoName)))
	if d := annotation.Description(string(m.Comments.Leading)); d != "" {
		summary, detail, _ := strings.Cut(d, "\n")
		add(op, "summary", str(summary))
		if detail = strings.TrimSpace(detail); detail != "" {
			add(op, "description", str(detail))
		}
	}
	add(op, "operationId", str(m.GoName))

	fields := rest.Partition(m.Input, h)
	var params []*yaml.Node
	for _, f := range fields.Path {
		// Handler は string / 整数 / bool の path param をその型に変換して読む（読めなければ 400）
		schema := mapping("type", str("string"))
		if rest.IsQueryable(f) {
			var err error
			if schema, err = g.fieldSchema(f, false); err != nil {
				return nil, err
			}
		}
		params = append(params, parameter(f, "path", true, schema))
	}
	for _, f := range fields.Query {
		s, err := g.fieldSchema(f, false)
		if err != nil {
			return nil, err
		}
		params = append(params, parameter(f, "query", false, s))
	}
	if len(params) > 0 {
		add(op, "parameters", seq(params...))
	}

	if len(fields.Body) > 0 {
		props := mapping()
		for _, f := range fields.Body {
			s, err := g.fieldSchema(f, false)
			if err != nil {
				return nil, err
			}
			add(props, string(f.Desc.Name()), withDescription(s, description(f)))
		}
		add(op, "requestBody", mapping(
			"required", boolean(true),
			"content", mapping("application/json", mapping("schema", mapping("type", str("object"), "properties", props))),
		))
	}

	shape := rest.ResponseShape(m.Output)
	status := rest.SuccessStatus(h.Method, shape)
	success := mapping("description", str(http.StatusText(status)))
	if shape.Kind != rest.ShapeEmpty {
		body := shape.Entity
		if shape.Kind == rest.ShapeOutput {
			body = m.Output
		}
		name, err := g.messageSchema(body)
		if err != nil {
			return nil, err
		}
		schema := ref(name)
		if shape.Kind == rest.ShapeList {
			schema = mapping("type", str("array"), "items", schema)
		}
		add(success, "content", mapping("application/json", mapping("schema", schema)))
	}
	add(op, "responses", mapping(
		strconv.Itoa(status), success,
		"default", fileRef(responseFile(problemResponse)),
	))
	return op, nil
}

func parameter(f *protogen.Field, in string, required bool, schema *yaml.Node) *yaml.Node {
	p := mapping("name", str(string(f.Desc.Name())), "in", str(in))
	if d := description(f); d != "" {
		add(p, "description", str(d))
	}
	if required {
		add(p, "required", boolean(true))
	}
	add(p, "schema", schema)
	return p
}

// messageSchema は m を component に登録して名前を返す。entity は Handler が返す DTO の形にする。
// DTO も Output struct も omitempty を持たないので、全フィールドを required とする。
func (g *generator) messageSchema(m *protogen.Message) (string, error) {
	name := m.GoIdent.GoName
	if c, ok := g.schemas[name]; ok {
		if c.origin != m.Desc.FullName() {
			return "", fmt.Errorf("component schema 名 %q が %s と %s で衝突している", name, c.origin, m.Desc.FullName())
		}
		return name, nil
	}
	c := &component{origin: m.Desc.FullName()}
	g.schemas[name] = c // 自己参照する message のため、中身を作る前に登録する

	isEntity := rest.IsEntity(m)
	props := mapping()
	var required []*yaml.Node
	for _, f := range m.Fields {
		if isEntity && rest.IsHidden(f) {
			continue
		}
		key := string(f.Desc.Name())
		var s *yaml.Node
		switch {
		case isEntity && annotation.HasMarker(string(f.Comments.Leading), "@timestamp"):
			key = rest.DTOFieldName(f)
			s = withDescription(mapping("type", str("integer"), "format", str("int64")),
				strings.TrimSpace(description(f)+"\nUnix 時刻（秒）"))
		default:
			if isEntity {
				key = rest.DTOFieldName(f)
			}
			var err error
			if s, err = g.fieldSchema(f, isEntity); err != nil {
				return "", err
			}
			s = withDescription(s, description(f))
		}
		add(props, key, s)
		required = append(required, str(key))
	}

	c.node = mapping("type", str("object"))
	if d := annotation.Description(string(m.Comments.Leading)); d != "" {
		add(c.node, "description", str(d))
	}
	if len(required) > 0 {
		add(c.node, "required", seq(required...))
	}
	add(c.node, "properties", props)
	return name, nil
}

// fieldSchema は 1 フィールドの JSON 表現（protojson ではなく encoding/json）のスキーマ。
// @decimal は entity（DTO）でだけ意味を持つので inEntity のときだけ解釈する。
func (g *generator) fieldSchema(f *protogen.Field, inEntity bool) (*yaml.Node, error) {
	if f.Desc.IsMap() {
		v, err := g.singleSchema(f.Message.Fields[1], false)
		if err != nil {
			return nil, err
		}
		return mapping("type", str("object"), "additionalProperties", v), nil
	}
	s, err := g.singleSchema(f, inEntity)
	if err != nil {
		return nil, err
	}
	if f.Desc.IsList() {
		return mapping("type", str("array"), "items", s), nil
	}
	return s, nil
}

func (g *generator) singleSchema(f *protogen.Field, inEntity bool) (*yaml.Node, error) {
	typ := func(t string) *yaml.Node { return mapping("type", str(t)) }
	typed := func(t, format string) *yaml.Node { return mapping("type", str(t), "format", str(format)) }
	switch f.Desc.Kind() {
	case protoreflect.StringKind:
		if inEntity {
			if _, _, isDecimal, _ := annotation.ParseDecimal(string(f.Comments.Leading)); isDecimal {
				return typed("string", "decimal"), nil // shopspring/decimal は JSON 文字列になる
			}
		}
		if annotation.HasMarker(string(f.Comments.Leading), "@email") {
			return typed("string", "email"), nil
		}
		return typ("string"), nil
	case protoreflect.BoolKind:
		return typ("boolean"), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return typed("integer", "int32"), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return typed("integer", "int64"), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return mapping("type", str("integer"), "minimum", integer(0)), nil
	case protoreflect.FloatKind:
		return typed("number", "float"), nil
	case protoreflect.DoubleKind:
		return typed("number", "double"), nil
	case protoreflect.BytesKind:
		return typed("string", "byte"), nil
	case protoreflect.EnumKind:
		return typed("integer", "int32"), nil // 生成 Go 型は int32 ベースで数値として出る
	case protoreflect.MessageKind, protoreflect.GroupKind:
		name, err := g.messageSchema(f.Message)
		if err != nil {
			return nil, err
		}
		return ref(name), nil
	}
	return nil, fmt.Errorf("field %s: unsupported kind %s", f.Desc.FullName(), f.Desc.Kind())
}

func (g *generator) addProblemSchemas() {
	g.schemas[fieldErrorSchema] = &component{origin: problemOrigin, node: mapping(
		"type", str("object"),
		"required", seq(str("field"), str("message")),
		"properties", mapping(
			"field", mapping("type", str("string")),
			"message", mapping("type", str("string")),
			"meta", mapping("type", str("object"), "additionalProperties", boolean(true)),
		),
	)}
	g.schemas[problemSchema] = &component{origin: problemOrigin, node: mapping(
		"type", str("object"),
		"required", seq(str("type"), str("title"), str("status")),
		"properties", mapping(
			"type", mapping("type", str("string")),
			"title", mapping("type", str("string")),
			"status", mapping("type", str("integer")),
			"detail", mapping("type", str("string")),
			"instance", mapping("type", str("string")),
			"errors", mapping("type", str("array"), "items", ref(fieldErrorSchema)),
		),
	)}
}

func description(f *protogen.Field) string {
	return annotation.Description(string(f.Comments.Leading))
}

func withDescription(s *yaml.Node, d string) *yaml.Node {
	if d == "" {
		return s
	}
	if len(s.Content) >= 2 && s.Content[0].Value == "$ref" {
		// OpenAPI 3.0 では $ref の兄弟キーは無視されるため allOf で包む
		return mapping("description", str(d), "allOf", seq(s))
	}
	add(s, "description", str(d))
	return s
}

// pathFile は "/api/users/{id}" → "paths/api_users_{id}.gen.yaml"。
func pathFile(p string) string {
	name := strings.ReplaceAll(strings.Trim(p, "/"), "/", "_")
	if name == "" {
		name = "root"
	}
	return "paths/" + name + ".gen.yaml"
}

func schemaFile(name string) string   { return "components/schemas/" + name + ".gen.yaml" }
func responseFile(name string) string { return "components/responses/" + name + ".gen.yaml" }

// fileRef は out からのパスで参照を作る。置き場所のファイルが決まった時点で encode が相対パスにする
// （同じ schema が別の階層のファイルから参照されるため、作る時点では相対パスを決められない）。
func fileRef(file string) *yaml.Node { return mapping("$ref", str(file)) }

func ref(schema string) *yaml.Node { return fileRef(schemaFile(schema)) }

// encode は name に置く node を YAML にする。$ref は name から見た相対パスにする。
func encode(node *yaml.Node, name string) ([]byte, error) {
	relativizeRefs(node, name)
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, err
	}
	err := enc.Close()
	return buf.Bytes(), err
}

func relativizeRefs(n *yaml.Node, from string) {
	if n.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(n.Content); i += 2 {
			if k, v := n.Content[i], n.Content[i+1]; k.Value == "$ref" && v.Kind == yaml.ScalarNode {
				v.Value = relPath(from, v.Value)
			}
		}
	}
	for _, c := range n.Content {
		relativizeRefs(c, from)
	}
}

func relPath(from, to string) string {
	rel, err := filepath.Rel(filepath.Dir(from), to)
	if err != nil {
		panic(err) // どちらも out からの相対パスなので起きない
	}
	if rel = filepath.ToSlash(rel); !strings.HasPrefix(rel, "../") {
		rel = "./" + rel
	}
	return rel
}

// mapping は yaml.Node を直接組み立てる（map を Marshal するとキー順が固定されないため）。
func mapping(kv ...any) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i < len(kv); i += 2 {
		add(n, kv[i].(string), kv[i+1].(*yaml.Node))
	}
	return n
}

func add(m *yaml.Node, key string, value *yaml.Node) {
	m.Content = append(m.Content, str(key), value)
}

func seq(items ...*yaml.Node) *yaml.Node {
	return &yaml.Node{Kind: yaml.SequenceNode, Content: items}
}

func str(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

func integer(i int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(i)}
}

func boolean(b bool) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(b)}
}
