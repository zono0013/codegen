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
