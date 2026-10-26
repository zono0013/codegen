package gogen

import (
	"strings"
	"testing"

	"github.com/zono0013/codegen/internal/prototest"
)

const testProto = `syntax = "proto3";
package shop.v1;
// @entity
message Item {
  // @pk
  string id = 1;
}
message GetItemRequest { string id = 1; }
message GetItemResponse { Item item = 1; }
// entity を入力に持つ rpc（Handler / Usecase interface が entity を import する分岐を通すため）
message ImportItemsRequest { repeated Item items = 1; }
message ImportItemsResponse {}
service ItemService {
  // @http GET /api/items/{id}
  rpc GetItem(GetItemRequest) returns (GetItemResponse);
  // @http POST /api/items/import
  rpc ImportItems(ImportItemsRequest) returns (ImportItemsResponse);
}
`

func TestGenerate_ImportsUseImportPath(t *testing.T) {
	const importPath = "example.com/shop/backend"
	files := prototest.Generate(t, map[string]string{"shop/v1/shop.proto": testProto}, New(Params{ImportPath: importPath}))

	imported := 0
	for name, src := range files {
		for _, line := range strings.Split(src, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, `"`) || !strings.Contains(line, "/internal/") && !strings.Contains(line, "/pkg/") {
				continue
			}
			imported++
			if !strings.HasPrefix(line, `"`+importPath+"/") {
				t.Errorf("%s: %s が %s 配下を指していない", name, line, importPath)
			}
		}
	}
	if imported == 0 {
		t.Fatal("生成物にプロジェクト内の import が 1 つも無い（テストの前提が崩れている）")
	}
}

func TestNew_RequiresImportPath(t *testing.T) {
	_, err := New(Params{})(prototest.Files(t, map[string]string{"shop/v1/shop.proto": testProto}))
	if err == nil || !strings.Contains(err.Error(), "ImportPath") {
		t.Errorf("err = %v, want ImportPath が空であることを示すエラー", err)
	}
}

func TestGenerate_Runtime(t *testing.T) {
	const importPath = "example.com/shop/backend"
	files := prototest.Generate(t, map[string]string{"shop/v1/shop.proto": testProto}, New(Params{ImportPath: importPath}))

	for _, name := range []string{
		"pkg/util/errs/errors.gen.go",
		"pkg/util/errs/http.gen.go",
		"pkg/util/tx/tx.gen.go",
		"internal/infra/postgres/errors.gen.go",
		"cmd/migration-gen/main.gen.go",
	} {
		if _, ok := files[name]; !ok {
			t.Errorf("%s が生成されていない", name)
		}
	}
	for name, src := range files {
		if strings.Contains(src, importPathPlaceholder) {
			t.Errorf("%s: %s が置き換わっていない", name, importPathPlaceholder)
		}
	}
	if src := files["cmd/migration-gen/main.gen.go"]; !strings.Contains(src, `"`+importPath+`/internal/domain/entity"`) {
		t.Errorf("migration-gen が生成先の entity を import していない:\n%s", src)
	}
}

func TestGenerate_NoEntity(t *testing.T) {
	// migration-gen（常に生成する）が entity.All を参照するので、entity が無くても registry を出す
	files := prototest.Generate(t, map[string]string{"empty/v1/empty.proto": `syntax = "proto3";
package empty.v1;
message Ping {}
`}, New(Params{ImportPath: "example.com/x/server"}))
	if src, ok := files["internal/domain/entity/registry.gen.go"]; !ok || !strings.Contains(src, "var All = []any{") {
		t.Errorf("entity が無いと registry.gen.go が出ない:\n%s", src)
	}
}

func TestGenerate_RuntimeOnlyFromSources(t *testing.T) {
	files := prototest.Generate(t, map[string]string{"shop/v1/shop.proto": testProto}, New(Params{ImportPath: "example.com/x/server"}))
	for name := range files {
		if !strings.HasSuffix(name, ".gen.go") {
			t.Errorf("%s: .gen.go 以外が出力された", name)
		}
	}
}

func TestGenerate_IntegerPK(t *testing.T) {
	// 整数の主キーは DB が採番する（0 は「未採番」）ので、Entity の生成関数で空チェックしない
	files := prototest.Generate(t, map[string]string{"tag/v1/tag.proto": `syntax = "proto3";
package tag.v1;
// @entity
message Tag {
  // @pk
  int64 id = 1;
  // @required
  string name = 2;
}
`}, New(Params{ImportPath: "example.com/x/server"}))
	src := files["internal/domain/entity/tag.gen.go"]
	if strings.Contains(src, `id == ""`) {
		t.Errorf("整数の主キーを文字列として検査している:\n%s", src)
	}
	if !strings.Contains(src, `name == ""`) {
		t.Errorf("@required の検査が消えている:\n%s", src)
	}
}

func TestGenerate_StringPKIsRequired(t *testing.T) {
	files := prototest.Generate(t, map[string]string{"shop/v1/shop.proto": testProto}, New(Params{ImportPath: "example.com/x/server"}))
	if src := files["internal/domain/entity/item.gen.go"]; !strings.Contains(src, `if id == "" {`) {
		t.Errorf("文字列の主キーの空チェックが無い:\n%s", src)
	}
}

const typedParamsProto = `syntax = "proto3";
package tag.v1;
// @entity
message Tag {
  // @pk
  int64 id = 1;
}
message GetTagRequest {
  int64 id = 1;
  int32 rev = 2;
  uint32 page = 3;
  uint64 offset = 4;
  bool archived = 5;
  string q = 6;
}
message GetTagResponse { Tag tag = 1; }
message DeleteTagRequest {
  uint64 id = 1;
  bool force = 2;
}
message DeleteTagResponse {}
service TagService {
  // @http GET /api/tags/{id}/revisions/{rev}
  rpc GetTag(GetTagRequest) returns (GetTagResponse);
  // @http DELETE /api/tags/{id}/{force}
  rpc DeleteTag(DeleteTagRequest) returns (DeleteTagResponse);
}
`

func TestGenerate_TypedParams(t *testing.T) {
	files := prototest.Generate(t, map[string]string{"tag/v1/tag.proto": typedParamsProto}, New(Params{ImportPath: "example.com/x/server"}))
	src := files["internal/handler/tag_handler.gen.go"]
	for _, want := range []string{
		// path は型に合わせて変換し、読めなければ 400 にする
		`_id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)`,
		`_rev, err := strconv.ParseInt(r.PathValue("rev"), 10, 32)`,
		`errs.NewBadRequestError("invalid path parameter: id")`,
		`_id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)`,
		`_force, err := strconv.ParseBool(r.PathValue("force"))`,
		`int32(_rev),`,
		// query の符号なし整数は同じ型で読む（int32 / int64 のまま代入するとコンパイルできない）
		`_page, _ := strconv.ParseUint(urlQuery.Get("page"), 10, 32)`,
		`uint32(_page),`,
		`_offset, _ := strconv.ParseUint(urlQuery.Get("offset"), 10, 64)`,
		`_archived, _ := strconv.ParseBool(urlQuery.Get("archived"))`,
		`_q := urlQuery.Get("q")`,
		`"strconv"`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("%q が無い:\n%s", want, src)
		}
	}
	if strings.Contains(src, `: r.PathValue(`) {
		t.Errorf("整数の path param を文字列のまま代入している:\n%s", src)
	}
}

func TestGenerate_StringPathParamNeedsNoStrconv(t *testing.T) {
	files := prototest.Generate(t, map[string]string{"shop/v1/shop.proto": testProto}, New(Params{ImportPath: "example.com/x/server"}))
	src := files["internal/handler/item_handler.gen.go"]
	if !strings.Contains(src, `: r.PathValue("id"),`) || strings.Contains(src, `"strconv"`) {
		t.Errorf("文字列の path param はそのまま読み、strconv を import しない:\n%s", src)
	}
}

func TestGenerate_OutputEntityIsDTO(t *testing.T) {
	// Output の中の entity も DTO にする。entity には json タグが無いので、そのまま JSON にすると
	// キーが Go のフィールド名（ID / Name）になり、OpenAPI・TS のスキーマ（snake_case）と食い違う
	files := prototest.Generate(t, map[string]string{"tag/v1/tag.proto": `syntax = "proto3";
package tag.v1;
// @entity
message Tag {
  // @pk
  int64 id = 1;
  string name = 2;
}
message TagGroup {
  string label = 1;
  Tag top = 2;
  map<string, Tag> by_name = 3;
}
message GetRandomTagsRequest {}
message GetRandomTagsResponse {
  string message = 1;
  repeated Tag tags = 2;
  TagGroup group = 3;
}
service TagService {
  // @http GET /api/tags/random
  rpc GetRandomTags(GetRandomTagsRequest) returns (GetRandomTagsResponse);
}
`}, New(Params{ImportPath: "example.com/x/server"}))
	src := files["internal/usecase/tag_usecase_interface.gen.go"]
	// テストでは gofmt を通さないので、空白の数は見ない
	flat := strings.Join(strings.Fields(src), " ")
	for _, want := range []string{
		"Tags []*dto.TagDTO `json:\"tags\"`",
		"Top *dto.TagDTO",
		"ByName map[string]*dto.TagDTO",
		`"example.com/x/server/internal/dto"`,
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("%q が無い:\n%s", want, src)
		}
	}
	if strings.Contains(src, "entity.") {
		t.Errorf("Output が entity を参照している:\n%s", src)
	}
}
