package openapigen

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"

	"github.com/zono0013/codegen/internal/prototest"
)

const testProto = `syntax = "proto3";
package shop.v1;
// 商品。
//
// @entity
message Item {
  // @pk
  string id = 1;
  // 表示名。
  // @required
  string name = 2;
  // @decimal(10,2)
  string price = 3;
  repeated string tags = 4;
  // @timestamp
  int64 created_at_unix = 5;
}

message Summary {
  int64 total = 1;
  map<string, int32> by_tag = 2;
}

message ListItemsRequest {
  int32 limit = 1;
  string after_id = 2;
}
message ListItemsResponse { repeated Item items = 1; }

message GetItemRequest { string id = 1; }
message GetItemResponse { Item item = 1; }

message CreateItemRequest {
  string name = 1;
  string price = 2;
}
message CreateItemResponse { Item item = 1; }

message RenameItemRequest {
  string id = 1;
  string name = 2;
}
message RenameItemResponse { Item item = 1; }

message DeleteItemRequest { string id = 1; }
message DeleteItemResponse {}

message SummarizeRequest {}
message SummarizeResponse {
  Summary summary = 1;
  int64 count = 2;
}

message InternalRequest {}
message InternalResponse {}

service ItemService {
  // 商品一覧を取得する。
  // @http GET /api/items
  rpc ListItems(ListItemsRequest) returns (ListItemsResponse);
  // @http GET /api/items/{id}
  rpc GetItem(GetItemRequest) returns (GetItemResponse);
  // @http POST /api/items
  rpc CreateItem(CreateItemRequest) returns (CreateItemResponse);
  // @http PATCH /api/items/{id}
  rpc RenameItem(RenameItemRequest) returns (RenameItemResponse);
  // @http DELETE /api/items/{id}
  rpc DeleteItem(DeleteItemRequest) returns (DeleteItemResponse);
  // @http GET /api/items/summary
  rpc Summarize(SummarizeRequest) returns (SummarizeResponse);
  // @http が無い rpc は Handler が登録されないので仕様にも載せない
  rpc Internal(InternalRequest) returns (InternalResponse);
}
`

func generateFiles(t *testing.T) map[string]string {
	t.Helper()
	return prototest.Generate(t, map[string]string{"shop/v1/shop.proto": testProto}, New(Params{Title: "Shop API", Version: "1.2.3"}))
}

func load(t *testing.T) *openapi3.T {
	t.Helper()
	return prototest.LoadOpenAPI(t, generateFiles(t), RootFile)
}

func TestGenerate_SplitFiles(t *testing.T) {
	files := generateFiles(t)

	t.Run("path ごと・schema ごとにファイルが分かれる", func(t *testing.T) {
		for _, want := range []string{
			RootFile,
			"paths/api_items.gen.yaml",
			"paths/api_items_{id}.gen.yaml",
			"paths/api_items_summary.gen.yaml",
			"components/schemas/Item.gen.yaml",
			"components/schemas/Summary.gen.yaml",
			"components/schemas/SummarizeResponse.gen.yaml",
			"components/schemas/ProblemDetails.gen.yaml",
			"components/schemas/FieldError.gen.yaml",
			"components/responses/Problem.gen.yaml",
		} {
			if _, ok := files[want]; !ok {
				t.Errorf("%s が無い（生成物: %v）", want, slices.Sorted(maps.Keys(files)))
			}
		}
	})

	t.Run("ルートは各ファイルへの $ref を繋ぐだけ", func(t *testing.T) {
		var root struct {
			Paths      map[string]map[string]any `yaml:"paths"`
			Components struct {
				Schemas   map[string]map[string]any `yaml:"schemas"`
				Responses map[string]map[string]any `yaml:"responses"`
			} `yaml:"components"`
		}
		if err := yaml.Unmarshal([]byte(files[RootFile]), &root); err != nil {
			t.Fatal(err)
		}
		onlyRef := func(kind, name string, v map[string]any, wantPrefix string) {
			ref, _ := v["$ref"].(string)
			if len(v) != 1 || !strings.HasPrefix(ref, wantPrefix) {
				t.Errorf("%s %s: $ref だけのはず: %v", kind, name, v)
			}
			if _, ok := files[strings.TrimPrefix(ref, "./")]; !ok {
				t.Errorf("%s %s: 参照先 %s が無い", kind, name, ref)
			}
		}
		for p, v := range root.Paths {
			onlyRef("path", p, v, "./paths/")
		}
		for n, v := range root.Components.Schemas {
			onlyRef("schema", n, v, "./components/schemas/")
		}
		for n, v := range root.Components.Responses {
			onlyRef("response", n, v, "./components/responses/")
		}
		if len(root.Paths) == 0 || len(root.Components.Schemas) == 0 {
			t.Errorf("paths / schemas が空")
		}
	})

	t.Run("各ファイルの $ref はファイル相対（単体でも辿れる）", func(t *testing.T) {
		for name, src := range files {
			if strings.Contains(src, "#/components/") {
				t.Errorf("%s: ルート内参照（#/components/...）が残っている", name)
			}
		}
	})
}

func op(t *testing.T, doc *openapi3.T, method, path string) *openapi3.Operation {
	t.Helper()
	item := doc.Paths.Find(path)
	if item == nil {
		t.Fatalf("path %s が無い", path)
	}
	o := item.GetOperation(method)
	if o == nil {
		t.Fatalf("%s %s が無い", method, path)
	}
	return o
}

func paramNames(o *openapi3.Operation, in string) []string {
	var out []string
	for _, p := range o.Parameters {
		if p.Value.In == in {
			out = append(out, p.Value.Name)
		}
	}
	return out
}

func refOf(t *testing.T, o *openapi3.Operation, status int) *openapi3.SchemaRef {
	t.Helper()
	r := o.Responses.Status(status)
	if r == nil || r.Value == nil {
		t.Fatalf("%s: %d レスポンスが無い", o.OperationID, status)
	}
	mt := r.Value.Content.Get("application/json")
	if mt == nil {
		t.Fatalf("%s: %d に application/json が無い", o.OperationID, status)
	}
	return mt.Schema
}

func TestGenerate_Operations(t *testing.T) {
	doc := load(t)

	t.Run("GET 一覧: path 以外の scalar は query、entity の repeated は DTO 配列を 200 で返す", func(t *testing.T) {
		o := op(t, doc, "GET", "/api/items")
		if o.OperationID != "ListItems" || !slices.Equal(o.Tags, []string{"ItemService"}) || o.Summary != "商品一覧を取得する。" {
			t.Errorf("operationId/tags/summary = %q/%v/%q", o.OperationID, o.Tags, o.Summary)
		}
		if got := paramNames(o, "query"); len(got) != 2 || got[0] != "limit" || got[1] != "after_id" {
			t.Errorf("query params = %v", got)
		}
		s := refOf(t, o, 200)
		if !s.Value.Type.Is("array") || !strings.HasSuffix(s.Value.Items.Ref, "schemas/Item.gen.yaml") {
			t.Errorf("200 schema = %+v", s.Value)
		}
	})

	t.Run("GET 単体: path param は必須の string、単一 entity は DTO を返す", func(t *testing.T) {
		o := op(t, doc, "GET", "/api/items/{id}")
		p := o.Parameters.GetByInAndName("path", "id")
		if p == nil || !p.Required || !p.Schema.Value.Type.Is("string") {
			t.Fatalf("path param id = %+v", p)
		}
		if got := refOf(t, o, 200).Ref; !strings.HasSuffix(got, "schemas/Item.gen.yaml") {
			t.Errorf("200 schema ref = %q", got)
		}
	})

	t.Run("POST: body に request の全フィールド、201 で返す", func(t *testing.T) {
		o := op(t, doc, "POST", "/api/items")
		if o.Responses.Status(200) != nil || o.Responses.Status(201) == nil {
			t.Errorf("POST は 201 のみのはず")
		}
		body := o.RequestBody.Value.Content.Get("application/json").Schema.Value
		if len(body.Properties) != 2 || body.Properties["name"] == nil || body.Properties["price"] == nil {
			t.Errorf("body properties = %v", slices.Sorted(maps.Keys(body.Properties)))
		}
	})

	t.Run("PATCH: path param は body から除く", func(t *testing.T) {
		o := op(t, doc, "PATCH", "/api/items/{id}")
		body := o.RequestBody.Value.Content.Get("application/json").Schema.Value
		if len(body.Properties) != 1 || body.Properties["name"] == nil {
			t.Errorf("body properties = %v", slices.Sorted(maps.Keys(body.Properties)))
		}
	})

	t.Run("DELETE: 空 response は body なしの 204、request body も無い", func(t *testing.T) {
		o := op(t, doc, "DELETE", "/api/items/{id}")
		r := o.Responses.Status(204)
		if r == nil || len(r.Value.Content) != 0 {
			t.Errorf("204 = %+v", r)
		}
		if o.RequestBody != nil {
			t.Errorf("DELETE に requestBody がある")
		}
	})

	t.Run("Output 形状: Response message をそのままのオブジェクトとして返す", func(t *testing.T) {
		o := op(t, doc, "GET", "/api/items/summary")
		if got := refOf(t, o, 200).Ref; !strings.HasSuffix(got, "schemas/SummarizeResponse.gen.yaml") {
			t.Errorf("200 schema ref = %q", got)
		}
		s := doc.Components.Schemas["Summary"]
		if s == nil {
			t.Fatalf("参照される非 entity message Summary が components に無い")
		}
		byTag := s.Value.Properties["by_tag"].Value
		if !byTag.Type.Is("object") || !byTag.AdditionalProperties.Schema.Value.Type.Is("integer") {
			t.Errorf("map は additionalProperties のはず: %+v", byTag)
		}
	})

	t.Run("全 operation が Problem Details のエラー応答を持つ", func(t *testing.T) {
		for path, item := range doc.Paths.Map() {
			for method, o := range item.Operations() {
				d := o.Responses.Default()
				if d == nil || !strings.HasSuffix(d.Ref, "responses/Problem.gen.yaml") {
					t.Errorf("%s %s: default response = %+v", method, path, d)
				}
			}
		}
		p := doc.Components.Responses["Problem"]
		if p == nil || p.Value.Content.Get("application/problem+json") == nil {
			t.Errorf("components.responses.Problem が application/problem+json でない")
		}
	})

	t.Run("@http の無い rpc は載せない", func(t *testing.T) {
		for _, item := range doc.Paths.Map() {
			for _, o := range item.Operations() {
				if o.OperationID == "Internal" {
					t.Errorf("Internal が載っている")
				}
			}
		}
	})
}

func TestGenerate_Info(t *testing.T) {
	doc := load(t)
	if doc.Info.Title != "Shop API" || doc.Info.Version != "1.2.3" {
		t.Errorf("info = %q / %q", doc.Info.Title, doc.Info.Version)
	}
}

func TestGenerate_EntitySchema(t *testing.T) {
	doc := load(t)
	s := doc.Components.Schemas["Item"]
	if s == nil {
		t.Fatalf("entity Item が components に無い")
	}
	v := s.Value
	if v.Description != "商品。" {
		t.Errorf("description = %q（アノテーション行は除く）", v.Description)
	}
	// DTO は omitempty を持たないので全フィールドが常に出る
	if want := []string{"id", "name", "price", "tags", "created_at_unix"}; !slices.Equal(v.Required, want) {
		t.Errorf("required = %v, want %v", v.Required, want)
	}
	tests := []struct {
		field, typ, format string
	}{
		{"id", "string", ""},
		{"name", "string", ""},
		{"price", "string", "decimal"}, // shopspring/decimal は JSON 文字列
		{"created_at_unix", "integer", "int64"},
	}
	for _, tt := range tests {
		p := v.Properties[tt.field]
		if p == nil {
			t.Errorf("%s が無い", tt.field)
			continue
		}
		if !p.Value.Type.Is(tt.typ) || p.Value.Format != tt.format {
			t.Errorf("%s = %v/%q, want %s/%q", tt.field, p.Value.Type, p.Value.Format, tt.typ, tt.format)
		}
	}
	if p := v.Properties["name"]; p != nil && p.Value.Description != "表示名。" {
		t.Errorf("name.description = %q", p.Value.Description)
	}
	if tags := v.Properties["tags"]; tags == nil || !tags.Value.Type.Is("array") || !tags.Value.Items.Value.Type.Is("string") {
		t.Errorf("tags は string の配列のはず")
	}
}
