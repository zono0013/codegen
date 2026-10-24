package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const valid = `proto: proto
targets:
  server:
    out: server
  front:
    out: front/generate
    client: "@/shared/api/apiClient"
  openapi:
    out: docs/openapi
    title: Shop API
    version: 1.0
`

func TestParse(t *testing.T) {
	got, err := Parse([]byte(valid))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{
		Proto: "proto",
		Targets: Targets{
			Server:  Server{Out: "server"},
			Front:   Front{Out: "front/generate", Client: "@/shared/api/apiClient"},
			OpenAPI: OpenAPI{Out: "docs/openapi", Title: "Shop API", Version: "1.0"},
		},
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParse_Invalid(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{"未知のキー", strings.Replace(valid, "proto: proto", "proto: proto\nprotos: x", 1), "protos"},
		{"必須の値が無い", strings.Replace(valid, "    title: Shop API\n", "", 1), "targets.openapi.title"},
		{"client に引用符", strings.Replace(valid, `client: "@/shared/api/apiClient"`, `client: "@/a'b"`, 1), "targets.front.client"},
		{"client が無い", strings.Replace(valid, "    client: \"@/shared/api/apiClient\"\n", "", 1), "targets.front.client"},
		{"proto が空", strings.Replace(valid, "proto: proto", "proto: \"\"", 1), "proto"},
		{"絶対パス", strings.Replace(valid, "out: server", "out: /tmp/server", 1), "targets.server.out"},
		{"ルートの外", strings.Replace(valid, "out: server", "out: ../server", 1), "targets.server.out"},
		{"出力先が同じ", strings.Replace(valid, "out: docs/openapi", "out: server", 1), "重なっている"},
		{"出力先が入れ子", strings.Replace(valid, "out: front/generate", "out: server/front", 1), "重なっている"},
		{"ルートそのもの", strings.Replace(valid, "out: server", "out: .", 1), "targets.server.out"},
		{"Clean するとルート", strings.Replace(valid, "out: server", "out: server/..", 1), "targets.server.out"},
		{"Clean するとルート（末尾 /）", strings.Replace(valid, "out: server", "out: ./", 1), "targets.server.out"},
		{"親ディレクトリ", strings.Replace(valid, "out: server", "out: ..", 1), "targets.server.out"},
		{"ドライブ指定", strings.Replace(valid, "out: server", "out: C:/server", 1), "targets.server.out"},
		{"途中で外に出て戻る", strings.Replace(valid, "out: server", "out: a/../../server", 1), "targets.server.out"},
		{"バックスラッシュ", strings.Replace(valid, "out: server", `out: 'a\b'`, 1), "targets.server.out"},
		{"空白だけ", strings.Replace(valid, "title: Shop API", `title: "  "`, 1), "targets.openapi.title"},
		{"空白だけのパス", strings.Replace(valid, "proto: proto", `proto: "  "`, 1), "proto"},
		{"空ファイル", "", "proto"},
		{"コメントだけ", "# empty\n", "proto"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want %q を含むエラー", err, tt.want)
			}
		})
	}
}

func TestLoad_Missing(t *testing.T) {
	_, err := Load(t.TempDir())
	if !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), FileName) {
		t.Errorf("err = %v, want %s が無いことを示す ErrNotExist", err, FileName)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoad(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, FileName), valid)
	writeFile(t, filepath.Join(root, "server", "go.mod"), "module example.com/shop/server\n\ngo 1.25\n")

	got, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Targets.OpenAPI.Title != "Shop API" {
		t.Errorf("got %+v", got)
	}
	if got.Targets.Server.ImportPath != "example.com/shop/server" {
		t.Errorf("ImportPath = %q, want server/go.mod の module", got.Targets.Server.ImportPath)
	}
}

func TestLoad_ImportPathFromParentModule(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, FileName), strings.Replace(valid, "out: server", "out: services/api", 1))
	writeFile(t, filepath.Join(root, "go.mod"), "// comment\nmodule \"example.com/shop\" // trailing\n")

	got, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := "example.com/shop/services/api"; got.Targets.Server.ImportPath != want {
		t.Errorf("ImportPath = %q, want %q", got.Targets.Server.ImportPath, want)
	}
}

func TestLoad_ImportPathFromIntermediateModule(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, FileName), strings.Replace(valid, "out: server", "out: services/api", 1))
	writeFile(t, filepath.Join(root, "services", "go.mod"), "module example.com/svc\n")

	got, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := "example.com/svc/api"; got.Targets.Server.ImportPath != want {
		t.Errorf("ImportPath = %q, want %q", got.Targets.Server.ImportPath, want)
	}
}

func TestModulePath(t *testing.T) {
	tests := []struct {
		name, gomod, want string
	}{
		{"通常", "module example.com/a\n\ngo 1.25\n", "example.com/a"},
		{"CRLF と行末コメント", "module example.com/a // Deprecated: x\r\n", "example.com/a"},
		{"引用符", "module \"example.com/a\"\n", "example.com/a"},
		{"ブロック", "module (\n\t// c\n\texample.com/block\n)\n", "example.com/block"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := modulePath([]byte(tt.gomod))
			if err != nil || got != tt.want {
				t.Errorf("got %q, %v, want %q", got, err, tt.want)
			}
		})
	}
	for name, gomod := range map[string]string{
		"無い":       "go 1.25\n",
		"値が無い":     "module\n",
		"空の引用符":    "module \"\"\n",
		"2 回":      "module example.com/a\nmodule example.com/b\n",
		"閉じないブロック": "module (\n\texample.com/a\n",
		"空のブロック":   "module (\n)\n",
	} {
		t.Run(name, func(t *testing.T) {
			if got, err := modulePath([]byte(gomod)); err == nil {
				t.Errorf("got %q, want エラー", got)
			}
		})
	}
}

func TestLoad_NoGoMod(t *testing.T) {
	parent := t.TempDir()
	// codegen.yaml のあるディレクトリより上の go.mod は使わない（別プロジェクトのものかもしれない）
	writeFile(t, filepath.Join(parent, "go.mod"), "module example.com/outer\n")
	root := filepath.Join(parent, "project")
	writeFile(t, filepath.Join(root, FileName), valid)

	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "go.mod") {
		t.Errorf("err = %v, want go.mod が見つからないことを示すエラー", err)
	}
}

func TestParse_NormalizesPaths(t *testing.T) {
	got, err := Parse([]byte(strings.Replace(valid, "out: front/generate", "out: ./front//generate/", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if got.Targets.Front.Out != "front/generate" {
		t.Errorf("Out = %q, want 正規化した front/generate", got.Targets.Front.Out)
	}
}

func TestParse_PrefixIsNotOverlap(t *testing.T) {
	// front と frontend は接頭辞が同じだけで、重なっていない
	yaml := strings.NewReplacer("out: docs/openapi", "out: frontend", "out: front/generate", "out: front").Replace(valid)
	if _, err := Parse([]byte(yaml)); err != nil {
		t.Errorf("err = %v", err)
	}
}

func TestLoad_OutInsideAnotherRepository(t *testing.T) {
	root := t.TempDir()
	// Submodule のチェックアウトは .git をファイルとして持つ
	writeFile(t, filepath.Join(root, "codegen", ".git"), "gitdir: ../.git/modules/codegen\n")
	writeFile(t, filepath.Join(root, "codegen", "example", "server", "go.mod"), "module example.com/x\n")
	writeFile(t, filepath.Join(root, FileName), strings.Replace(valid, "out: server", "out: codegen/example/server", 1))

	_, err := Load(root)
	if err == nil || !strings.Contains(err.Error(), "別の Git リポジトリ") {
		t.Errorf("err = %v, want Submodule の中への生成を拒むエラー", err)
	}
}

func TestLoad_RootItselfMayBeRepository(t *testing.T) {
	// 使う側のプロジェクトのルートは Git リポジトリ（.git はディレクトリ、それ自体が Submodule ならファイル）
	for name, write := range map[string]func(root string){
		"ディレクトリ": func(root string) { writeFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main\n") },
		"ファイル":   func(root string) { writeFile(t, filepath.Join(root, ".git"), "gitdir: ../.git/modules/app\n") },
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			write(root)
			writeFile(t, filepath.Join(root, FileName), valid)
			writeFile(t, filepath.Join(root, "server", "go.mod"), "module example.com/shop/server\n")
			if _, err := Load(root); err != nil {
				t.Errorf("ルートが Git リポジトリなだけで弾かれた: %v", err)
			}
		})
	}
}
