package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
service ItemService {
  // @http GET /api/items/{id}
  rpc GetItem(GetItemRequest) returns (GetItemResponse);
}
`

// testConfig は proto の場所と出力先をわざと既定の配置（proto / server / front/generate / docs/openapi）と変えておく。
const testConfig = `proto: schema
targets:
  server:
    out: backend
  front:
    out: web/src/gen
    client: "~/http"
  openapi:
    out: api
    title: Shop API
    version: 2.3.4
`

func newProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	p := filepath.Join(root, "schema", "shop", "v1", "shop.proto")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(testProto), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "codegen.yaml"), []byte(testConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "backend"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "backend", "go.mod"), []byte("module example.com/shop/backend\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func runGen(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(args, &out)
	return out.String(), err
}

func TestRun_GenerateThenCheck(t *testing.T) {
	root := newProject(t)
	if _, err := runGen(t, "-root", root); err != nil {
		t.Fatal(err)
	}
	if out, err := runGen(t, "check", "-root", root); err != nil {
		t.Fatalf("生成直後なのに check が落ちた: %v\n%s", err, out)
	}
}

func TestRun_CheckReportsStaleFile(t *testing.T) {
	root := newProject(t)
	if _, err := runGen(t, "generate", "-root", root); err != nil {
		t.Fatal(err)
	}
	entity := filepath.Join(root, "backend", "internal", "domain", "entity", "item.gen.go")
	src, err := os.ReadFile(entity)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entity, append(src, []byte("// edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runGen(t, "check", "-root", root)
	if !errors.Is(err, errReported) {
		t.Fatalf("err = %v, want errReported\n%s", err, out)
	}
	if !strings.Contains(out, "✗ server: internal/domain/entity/item.gen.go: 生成物が最新でない") {
		t.Errorf("編集した生成物が報告されていない:\n%s", out)
	}
	if after, err := os.ReadFile(entity); err != nil || !strings.HasSuffix(string(after), "// edited\n") {
		t.Errorf("check がファイルに触れた: err = %v", err)
	}
}

func TestRun_CleanRemovesOnlyGeneratedFiles(t *testing.T) {
	root := newProject(t)
	if _, err := runGen(t, "-root", root); err != nil {
		t.Fatal(err)
	}
	handwritten := filepath.Join(root, "backend", "internal", "usecase", "other.gen.go")
	if err := os.WriteFile(handwritten, []byte("package usecase\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := runGen(t, "clean", "-root", root); err != nil {
		t.Fatal(err)
	}
	var left []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.Contains(d.Name(), ".gen.") {
			left = append(left, p)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 1 || left[0] != handwritten {
		t.Errorf("残った *.gen.* = %v, want マーカーを持たない %s だけ", left, handwritten)
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	if _, err := runGen(t, "bogus"); err == nil || !strings.Contains(err.Error(), "不明なサブコマンド") {
		t.Errorf("err = %v", err)
	}
}

func TestRun_Help(t *testing.T) {
	if _, err := runGen(t, "check", "-h"); err != nil {
		t.Errorf("-h はエラーにしない: %v", err)
	}
}

func TestRun_BadFlagIsReportedOnce(t *testing.T) {
	// FlagSet が stderr に報告済みなので、main がもう一度出さないよう errReported を返す
	if _, err := runGen(t, "-bogus"); !errors.Is(err, errReported) {
		t.Errorf("err = %v, want errReported", err)
	}
}

func TestRun_FollowsConfig(t *testing.T) {
	root := newProject(t)
	if _, err := runGen(t, "-root", root); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{
		"backend/internal/domain/entity/item.gen.go",
		"web/src/gen/api/shop.gen.ts",
		"api/openapi.gen.yaml",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(p))); err != nil {
			t.Errorf("%s が生成されていない: %v", p, err)
		}
	}
	spec, err := os.ReadFile(filepath.Join(root, "api", "openapi.gen.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	di, err := os.ReadFile(filepath.Join(root, "backend", "internal", "di", "handlers.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(di), `"example.com/shop/backend/internal/handler"`) {
		t.Errorf("backend/go.mod の module が import に反映されていない:\n%s", di)
	}
	if !strings.Contains(string(spec), "title: Shop API") || !strings.Contains(string(spec), "version: 2.3.4") {
		t.Errorf("codegen.yaml の title / version が反映されていない:\n%s", spec)
	}
}

func TestRun_MissingConfig(t *testing.T) {
	root := newProject(t)
	if err := os.Remove(filepath.Join(root, "codegen.yaml")); err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []string{"generate", "check", "clean"} {
		if _, err := runGen(t, cmd, "-root", root); err == nil || !strings.Contains(err.Error(), "codegen.yaml") {
			t.Errorf("%s: err = %v, want codegen.yaml が無いことを示すエラー", cmd, err)
		}
	}
}
