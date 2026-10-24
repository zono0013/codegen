package check

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zono0013/codegen/internal/config"
	"github.com/zono0013/codegen/internal/pipeline"
	"github.com/zono0013/codegen/internal/prototest"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func messages(problems []Problem) string {
	var lines []string
	for _, p := range problems {
		lines = append(lines, p.String())
	}
	return strings.Join(lines, "\n")
}

func TestStale(t *testing.T) {
	root := t.TempDir()
	target := pipeline.Target{Name: "x", Out: "out", Suffix: ".gen.txt"}
	header := "// " + pipeline.Marker + " (test). DO NOT EDIT.\n"
	writeFile(t, filepath.Join(root, "out", "fresh.gen.txt"), header+"same\n")
	writeFile(t, filepath.Join(root, "out", "stale.gen.txt"), header+"old\n")
	writeFile(t, filepath.Join(root, "out", "orphan.gen.txt"), header+"gone\n")
	writeFile(t, filepath.Join(root, "out", "handwritten.gen.txt"), "not ours\n")

	rendered := map[string]map[string][]byte{"x": {
		"fresh.gen.txt":   []byte(header + "same\n"),
		"stale.gen.txt":   []byte(header + "new\n"),
		"missing.gen.txt": []byte(header + "new\n"),
	}}
	problems, err := Stale(root, []pipeline.Target{target}, rendered)
	if err != nil {
		t.Fatal(err)
	}

	got := messages(problems)
	for _, want := range []string{
		"x: stale.gen.txt: 生成物が最新でない",
		"line 2:\n  committed: old\n  generated: new",
		"x: missing.gen.txt: 生成物が無い",
		"x: orphan.gen.txt: 生成されない古い生成物が残っている",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q が報告されていない:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"fresh.gen.txt", "handwritten.gen.txt"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("%s は問題ではないのに報告された:\n%s", unwanted, got)
		}
	}
	if len(problems) != 3 {
		t.Errorf("問題の数 = %d, want 3:\n%s", len(problems), got)
	}
}

func TestStale_UpToDate(t *testing.T) {
	root := t.TempDir()
	target := pipeline.Target{Name: "x", Out: "out", Suffix: ".gen.txt"}
	rendered := map[string]map[string][]byte{"x": {"a.gen.txt": []byte("a\n")}}
	if _, err := pipeline.Write(root, target, rendered["x"]); err != nil {
		t.Fatal(err)
	}

	problems, err := Stale(root, []pipeline.Target{target}, rendered)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("最新なのに問題が報告された:\n%s", messages(problems))
	}
}

const routesProto = `syntax = "proto3";
package shop.v1;
// @entity
message Item {
  // @pk
  string id = 1;
}
message GetItemRequest { string id = 1; }
message GetItemResponse { Item item = 1; }
service ItemService {
  // @http GET /api/items/{id}
  rpc GetItem(GetItemRequest) returns (GetItemResponse);
}
`

var testConfig = config.Config{Proto: "proto", Targets: config.Targets{
	Server:  config.Server{Out: "server", ImportPath: "example.com/shop/server"},
	Front:   config.Front{Out: "front/generate", Client: "@/api"},
	OpenAPI: config.OpenAPI{Out: "docs/openapi", Title: "T", Version: "1"},
}}

func renderRoutesProto(t *testing.T) map[string]map[string][]byte {
	t.Helper()
	protos := prototest.Files(t, map[string]string{"shop/v1/shop.proto": routesProto})
	rendered, err := pipeline.RenderAll(protos, pipeline.Targets(testConfig))
	if err != nil {
		t.Fatal(err)
	}
	return rendered
}

func TestRoutes_Match(t *testing.T) {
	problems, err := Routes(renderRoutesProto(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("一致しているのに問題が報告された:\n%s", messages(problems))
	}
}

func TestRoutes_Mismatch(t *testing.T) {
	rendered := renderRoutesProto(t)
	di := string(rendered["server"][HandlersFile])
	registered := `mux.HandleFunc("GET /api/items/{id}", h.ItemService.GetItem)`
	if !strings.Contains(di, registered) {
		t.Fatalf("前提: %s に %s が無い:\n%s", HandlersFile, registered, di)
	}
	di = strings.Replace(di, registered, registered+"\n\tmux.HandleFunc(\"DELETE /api/items/{id}\", h.ItemService.DeleteItem)", 1)
	di = strings.Replace(di, `h.ItemService.GetItem)`, `h.ItemService.FetchItem)`, 1)
	rendered["server"][HandlersFile] = []byte(di)

	problems, err := Routes(rendered)
	if err != nil {
		t.Fatal(err)
	}
	got := messages(problems)
	for _, want := range []string{
		"DELETE /api/items/{id}: サーバーにあるが仕様に無い",
		`GET /api/items/{id}: operationId "GetItem" が Handler "FetchItem" と一致しない`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q が報告されていない:\n%s", want, got)
		}
	}
}

func TestRoutes_MissingAndDuplicate(t *testing.T) {
	rendered := renderRoutesProto(t)
	di := string(rendered["server"][HandlersFile])
	registered := `mux.HandleFunc("GET /api/items/{id}", h.ItemService.GetItem)`

	t.Run("仕様にあるがサーバーに無い", func(t *testing.T) {
		rendered["server"][HandlersFile] = []byte(strings.Replace(di, registered, `mux.HandleFunc("GET /api/other", h.ItemService.Other)`, 1))
		problems, err := Routes(rendered)
		if err != nil {
			t.Fatal(err)
		}
		if got := messages(problems); !strings.Contains(got, "openapi: GET /api/items/{id}: 仕様にあるがサーバーに無い") {
			t.Errorf("報告されていない:\n%s", got)
		}
	})

	t.Run("重複登録", func(t *testing.T) {
		rendered["server"][HandlersFile] = []byte(strings.Replace(di, registered, registered+"\n\t"+registered, 1))
		problems, err := Routes(rendered)
		if err != nil {
			t.Fatal(err)
		}
		if got := messages(problems); !strings.Contains(got, "同じルートが重複登録されている") {
			t.Errorf("報告されていない:\n%s", got)
		}
	})
}

func TestRoutes_NoService(t *testing.T) {
	protos := prototest.Files(t, map[string]string{"shop/v1/shop.proto": `syntax = "proto3";
package shop.v1;
// @entity
message Item {
  // @pk
  string id = 1;
}
`})
	rendered, err := pipeline.RenderAll(protos, pipeline.Targets(testConfig))
	if err != nil {
		t.Fatal(err)
	}
	problems, err := Routes(rendered)
	if err != nil || len(problems) != 0 {
		t.Errorf("service が無いだけなのに失敗した: err = %v, problems = %s", err, messages(problems))
	}
}
