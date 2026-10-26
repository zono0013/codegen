// Package tsgen は front/generate/ 配下の Zod スキーマ（schemas/）と型付き API クライアント（api/）を生成する。
// ファイルは proto/ 直下のディレクトリ（ドメイン）ごとに分け、index.gen.ts でまとめて re-export する。
package tsgen

import (
	"errors"
	"slices"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/zono0013/codegen/internal/annotation"
	"github.com/zono0013/codegen/internal/rest"
)

type route struct {
	method *protogen.Method
	http   *annotation.HTTP
}

type domain struct {
	name     string
	messages []*protogen.Message
	routes   []route
}

// Params は生成先のプロジェクトごとに変わる値。
type Params struct {
	// Client は API 関数が使う apiClient の import 指定子（例: "@/shared/api/apiClient"）。
	// 認証やリトライはプロジェクトの方針なので、apiClient 自体は生成せずプロジェクトが持つ。
	Client string
}

// New は params に従い、front の出力先からの相対パス → TypeScript コードを返すジェネレータを作る。
func New(params Params) func([]*protogen.File) (map[string][]byte, error) {
	return func(files []*protogen.File) (map[string][]byte, error) { return generate(files, params) }
}

func generate(files []*protogen.File, params Params) (map[string][]byte, error) {
	if params.Client == "" {
		return nil, errors.New("tsgen: Client が空")
	}
	files = slices.Clone(files)
	slices.SortFunc(files, func(a, b *protogen.File) int { return strings.Compare(a.Desc.Path(), b.Desc.Path()) })

	var domains []*domain
	byName := map[string]*domain{}
	for _, f := range files {
		name := domainOf(f.Desc.Path())
		d, ok := byName[name]
		if !ok {
			d = &domain{name: name}
			byName[name] = d
			domains = append(domains, d)
		}
		d.messages = append(d.messages, f.Messages...)
		for _, svc := range f.Services {
			for _, m := range svc.Methods {
				if h := annotation.ParseHTTP(string(m.Comments.Leading)); h != nil {
					d.routes = append(d.routes, route{m, h})
				}
			}
		}
	}

	out := map[string][]byte{}
	var schemaDomains, apiDomains []string
	for _, d := range domains {
		if len(d.messages) > 0 {
			out["schemas/"+d.name+".gen.ts"] = []byte(schemaFile(d, byName))
			schemaDomains = append(schemaDomains, d.name)
		}
		if len(d.routes) > 0 {
			out["api/"+d.name+".gen.ts"] = []byte(apiFile(d, params.Client))
			apiDomains = append(apiDomains, d.name)
		}
	}
	out["schemas/index.gen.ts"] = []byte(indexFile(schemaDomains))
	out["api/index.gen.ts"] = []byte(indexFile(apiDomains))
	return out, nil
}

func domainOf(protoPath string) string {
	return strings.ReplaceAll(strings.SplitN(protoPath, "/", 2)[0], "_", "-")
}

func name(m *protogen.Message) string { return string(m.Desc.Name()) }

// referencedMessage は f が指す message（map なら value 側）。scalar / enum なら nil。
func referencedMessage(f *protogen.Field) *protogen.Message {
	if f.Desc.IsMap() {
		f = f.Message.Fields[1]
	}
	if f.Desc.Kind() != protoreflect.MessageKind && f.Desc.Kind() != protoreflect.GroupKind {
		return nil
	}
	return f.Message
}

func zodType(f *protogen.Field) string {
	elem := f
	if f.Desc.IsMap() {
		elem = f.Message.Fields[1]
	}
	var z string
	switch elem.Desc.Kind() {
	case protoreflect.StringKind:
		z = "z.string()"
		if annotation.HasMarker(string(elem.Comments.Leading), "@email") {
			z = "z.string().email()"
		}
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind, protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		z = "z.number().int()"
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind,
		protoreflect.FloatKind, protoreflect.DoubleKind:
		z = "z.number()"
	case protoreflect.BoolKind:
		z = "z.boolean()"
	case protoreflect.BytesKind:
		z = "z.string()" // JSON では base64 文字列
	case protoreflect.EnumKind:
		z = string(elem.Enum.Desc.Name()) + "Schema"
	default:
		z = name(elem.Message) + "Schema"
	}
	switch {
	case f.Desc.IsMap():
		return "z.record(z.string(), " + z + ")"
	case f.Desc.IsList():
		return "z.array(" + z + ")"
	}
	return z
}

func schemaFile(d *domain, domains map[string]*domain) string {
	// Handler は Response をそのまま返さないので、FE の型も実際に返る形に合わせる
	shapes := map[protoreflect.FullName]rest.Shape{}
	for _, r := range d.routes {
		if s := rest.ResponseShape(r.method.Output); s.Kind != rest.ShapeOutput {
			shapes[r.method.Output.Desc.FullName()] = s
		}
	}

	lines := []string{"import { z } from 'zod'"}
	var deps []string
	depNames := map[string][]string{}
	for _, msg := range d.messages {
		for _, f := range msg.Fields {
			ref := referencedMessage(f)
			if ref == nil {
				continue
			}
			dep := domainOf(ref.Location.SourceFile)
			if _, generated := domains[dep]; !generated || dep == d.name {
				continue
			}
			if _, seen := depNames[dep]; !seen {
				deps = append(deps, dep)
			}
			if schema := name(ref) + "Schema"; !slices.Contains(depNames[dep], schema) {
				depNames[dep] = append(depNames[dep], schema)
			}
		}
	}
	for _, dep := range deps {
		lines = append(lines, "import { "+strings.Join(depNames[dep], ", ")+" } from './"+dep+".gen'")
	}
	lines = append(lines, "")

	for _, msg := range topoSort(d.messages) {
		n := name(msg)
		if s, ok := shapes[msg.Desc.FullName()]; ok {
			schema := "z.void()"
			switch s.Kind {
			case rest.ShapeList:
				schema = "z.array(" + name(s.Entity) + "Schema)"
			case rest.ShapeEntity:
				schema = name(s.Entity) + "Schema"
			}
			lines = append(lines, "export const "+n+"Schema = "+schema)
		} else {
			lines = append(lines, "export const "+n+"Schema = z.object({")
			if len(msg.Fields) == 0 {
				lines = append(lines, "  // empty message")
			}
			isEntity := rest.IsEntity(msg)
			for _, f := range msg.Fields {
				if isEntity && rest.IsHidden(f) {
					continue
				}
				lines = append(lines, "  "+string(f.Desc.Name())+": "+zodType(f)+",")
			}
			lines = append(lines, "})")
		}
		lines = append(lines, "export type "+n+" = z.infer<typeof "+n+"Schema>", "")
	}
	return strings.Join(lines, "\n")
}

// topoSort は const の参照先が先に宣言されるよう、同じドメインの参照先を先に並べる（それ以外は宣言順）。
func topoSort(messages []*protogen.Message) []*protogen.Message {
	inDomain := map[*protogen.Message]bool{}
	for _, m := range messages {
		inDomain[m] = true
	}
	var sorted []*protogen.Message
	seen := map[*protogen.Message]bool{}
	var visit func(*protogen.Message)
	visit = func(m *protogen.Message) {
		if seen[m] || !inDomain[m] {
			return
		}
		seen[m] = true
		for _, f := range m.Fields {
			if ref := referencedMessage(f); ref != nil {
				visit(ref)
			}
		}
		sorted = append(sorted, m)
	}
	for _, m := range messages {
		visit(m)
	}
	return sorted
}

func apiFile(d *domain, client string) string {
	var types []string
	add := func(t string) {
		if !slices.Contains(types, t) {
			types = append(types, t)
		}
	}
	for _, r := range d.routes {
		if len(r.method.Input.Fields) > 0 { // 空 request はシグネチャに出ない
			add(name(r.method.Input))
		}
	}
	for _, r := range d.routes {
		add(name(r.method.Output))
	}

	lines := []string{
		"import type { " + strings.Join(types, ", ") + " } from '../schemas/" + d.name + ".gen'",
		"import { apiClient } from '" + client + "'",
		"",
	}
	for _, r := range d.routes {
		lines = append(lines, apiFunction(r))
	}
	return strings.Join(lines, "\n") + "\n"
}

func apiFunction(r route) string {
	req, res := name(r.method.Input), name(r.method.Output)
	pathParams := annotation.PathParams(r.http.Path)
	url := "'" + r.http.Path + "'"
	if len(pathParams) > 0 {
		tpl := r.http.Path
		for _, p := range pathParams {
			tpl = strings.Replace(tpl, "{"+p+"}", "${"+p+"}", 1)
		}
		url = "`" + tpl + "`"
	}
	call := "apiClient." + strings.ToLower(r.http.Method) + "<" + res + ">(" + url
	isQuery := rest.IsQueryMethod(r.http.Method)
	destructured := "{ " + strings.Join(pathParams, ", ")

	var signature string
	switch {
	case len(r.method.Input.Fields) == 0:
		signature, call = "()", call+")"
	case len(pathParams) == 0 && isQuery:
		signature, call = "(params: "+req+")", call+", { params })"
	case len(pathParams) == 0:
		signature, call = "(data: "+req+")", call+", data)"
	case len(pathParams) == len(r.method.Input.Fields):
		signature, call = "("+destructured+" }: "+req+")", call+")"
	case isQuery:
		signature, call = "("+destructured+", ...rest }: "+req+")", call+", { params: rest })"
	default:
		signature, call = "("+destructured+", ...data }: "+req+")", call+", data)"
	}
	fn := strings.ToLower(r.method.GoName[:1]) + r.method.GoName[1:]
	return "export const " + fn + " = " + signature + ": Promise<" + res + "> =>\n  " + call + ".then(r => r.data)"
}

func indexFile(domains []string) string {
	var exports []string
	for _, d := range domains {
		exports = append(exports, "export * from './"+d+".gen'")
	}
	return strings.Join(exports, "\n") + "\n"
}
