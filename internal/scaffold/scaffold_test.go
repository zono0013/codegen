package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zono0013/codegen/internal/pipeline"
)

func TestRewriteCodegenPaths(t *testing.T) {
	tests := []struct {
		name  string
		depth int
		in    string
		want  string
	}{
		{"リンク", 1, "[spec](../../docs/spec.md)", "[spec](../codegen/docs/spec.md)"},
		{"末尾スラッシュ無し", 1, "make -C ../.. proto-gen", "make -C ../codegen proto-gen"},
		{"バッククォートで閉じる", 1, "codegen（`../../`）", "codegen（`../codegen/`）"},
		{"深いファイル", 2, "仕様は `../../../docs/spec.md`", "仕様は `../../codegen/docs/spec.md`"},
		{"codegen より上は触らない", 1, "../../../other", "../../../other"},
		{"同じ階層の親は触らない", 1, "../proto/a.proto", "../proto/a.proto"},
		{"深さが違う相対パスは触らない", 2, "../../server", "../../server"},
		{"@import", 1, "@../../docs/front/x/", "@../codegen/docs/front/x/"},
		{"隠しディレクトリ", 1, "../../.github/workflows", "../codegen/.github/workflows"},
		{"文末のピリオド", 1, "see ../... ", "see ../... "},
		{"文末のピリオド（codegen）", 1, "see ../... and ../..", "see ../... and ../codegen"},
		{"英文のピリオド", 1, "under ../.., then", "under ../codegen, then"},
		{"名前の一部", 1, "../..foo", "../..foo"},
		{"depth 0 の数値範囲", 0, "1..10 と a..b", "1..10 と a..b"},
		{"depth 0 のリンク", 0, "仕様は ../docs/spec.md", "仕様は codegen/docs/spec.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rewriteCodegenPaths(tt.in, tt.depth, "codegen"); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// newSample は Git 管理下のサンプルを作る（Copy は git ls-files で対象を選ぶ）。
func newSample(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(name, ".sh") {
			mode = 0o755
		}
		if err := os.WriteFile(p, []byte(content), mode); err != nil {
			t.Fatal(err)
		}
	}
	// codegen の中を指すシンボリックリンク（使う側では codegen/ を指すように書き換わる）
	if err := os.Symlink("../../docs", filepath.Join(dir, "server", "docs-link")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "-A"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func TestCopy(t *testing.T) {
	from := newSample(t, map[string]string{
		"server/go.mod":    "module example.com/blog/server\n",
		"server/main.go":   "package main\n\nimport _ \"example.com/blog/server/internal/x\"\nimport _ \"example.com/blog/serverless\"\n",
		"server/x.gen.go":  "// " + pipeline.Marker + " (x). DO NOT EDIT.\npackage x\n",
		"server/run.sh":    "#!/bin/sh\n",
		"server/mine.txt":  "theirs\n",
		"server/CLAUDE.md": "[spec](../../docs/spec.md)\n",
		"codegen.yaml":     "proto: proto\n",
	})
	to := t.TempDir()
	if err := os.MkdirAll(filepath.Join(to, "server"), 0o755); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(to, "server", "mine.txt")
	if err := os.WriteFile(mine, []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{From: from, To: to, CodegenDir: "codegen", FromModule: "example.com/blog/server", ToModule: "example.com/acme/server"}

	res, err := Copy(opts)
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(to, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	if got := read("server/main.go"); !strings.Contains(got, `"example.com/acme/server/internal/x"`) || !strings.Contains(got, `"example.com/blog/serverless"`) {
		t.Errorf("module の置き換えが違う（別の module 名の一部は置き換えない）:\n%s", got)
	}
	if got := read("server/CLAUDE.md"); got != "[spec](../codegen/docs/spec.md)\n" {
		t.Errorf("CLAUDE.md = %q", got)
	}
	for _, name := range []string{"server/x.gen.go", "codegen.yaml"} {
		if _, err := os.Stat(filepath.Join(to, filepath.FromSlash(name))); err == nil {
			t.Errorf("%s をコピーした", name)
		}
	}
	if info, err := os.Stat(filepath.Join(to, "server", "run.sh")); err != nil || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("実行ビットが落ちた: %v %v", info, err)
	}
	if read("server/mine.txt") != "mine\n" || !slicesContains(res.Skipped, "server/mine.txt") {
		t.Errorf("既にあるファイルを上書きした、または Skipped に入っていない: %v", res.Skipped)
	}

	if link, err := os.Readlink(filepath.Join(to, "server", "docs-link")); err != nil || link != filepath.FromSlash("../codegen/docs") {
		t.Errorf("シンボリックリンクの向き先 = %q, %v", link, err)
	}

	opts.Force = true
	if _, err := Copy(opts); err != nil {
		t.Fatal(err)
	}
	if read("server/mine.txt") != "theirs\n" {
		t.Errorf("-force でも上書きしない")
	}
}

func slicesContains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
