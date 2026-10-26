package main

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newConsumer は codegen を codegen/ に置いた（Submodule として取り込んだのと同じ配置の）プロジェクトを作る。
func newConsumer(t *testing.T) (root, codegen string) {
	t.Helper()
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	root = filepath.Join(t.TempDir(), "acme")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	codegen = filepath.Join(root, "codegen")
	if err := os.Symlink(repo, codegen); err != nil {
		t.Fatal(err)
	}
	return root, codegen
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRun_Init(t *testing.T) {
	root, codegen := newConsumer(t)
	out, err := runGen(t, "init", "-root", root, "-codegen", codegen, "-module", "example.com/acme/server")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	cfg := read(t, filepath.Join(root, "codegen.yaml"))
	for _, want := range []string{`title: "acme API"`, `version: "0.1.0"`, "codegen/docs/spec.md"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("codegen.yaml に %q が無い:\n%s", want, cfg)
		}
	}
	if got := read(t, filepath.Join(root, "server", "go.mod")); !strings.HasPrefix(got, "module example.com/acme/server\n") {
		t.Errorf("server/go.mod の module が置き換わっていない:\n%s", got)
	}
	if got := read(t, filepath.Join(root, "front", "package.json")); !strings.Contains(got, `"gen": "make -C ../codegen proto-gen"`) {
		t.Errorf("front の gen が codegen を指していない:\n%s", got)
	}
	if got := read(t, filepath.Join(root, "server", "CLAUDE.md")); !strings.Contains(got, "../codegen/docs/spec.md") {
		t.Errorf("server/CLAUDE.md の仕様へのパスが codegen を指していない")
	}

	// サンプルの module 名がどこにも残っていない（生成物も手書きも）
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == codegen {
			return filepath.SkipDir
		}
		if d.Type().IsRegular() && strings.Contains(read(t, p), "example.com/blog/server") {
			t.Errorf("%s にサンプルの module 名が残っている", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if out, err := runGen(t, "check", "-root", root); err != nil {
		t.Errorf("init 直後の check が落ちた: %v\n%s", err, out)
	}

	if testing.Short() {
		return
	}
	build := exec.Command("go", "build", "./...")
	build.Dir = filepath.Join(root, "server")
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=-mod=mod")
	if out, err := build.CombinedOutput(); err != nil {
		t.Errorf("コピーしたサーバーがビルドできない: %v\n%s", err, out)
	}
}

func TestRun_InitDoesNotOverwrite(t *testing.T) {
	root, codegen := newConsumer(t)
	args := []string{"init", "-root", root, "-codegen", codegen, "-module", "example.com/acme/server"}
	if _, err := runGen(t, args...); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(root, "server", "cmd", "api", "main.go")
	if err := os.WriteFile(mine, []byte("package main\n\n// mine\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := runGen(t, args...); err == nil || !strings.Contains(err.Error(), "初期化済み") {
		t.Errorf("2 回目の init が codegen.yaml を黙って上書きした: err = %v", err)
	}
	if got := read(t, mine); !strings.Contains(got, "// mine") {
		t.Errorf("使う側のファイルを上書きした")
	}
}

func TestRun_InitChecksInitializedBeforeAsking(t *testing.T) {
	// 初期化済みなら、module などを聞く（必要とする）前に止める。答えてから断られないように
	root, codegen := newConsumer(t)
	if err := os.WriteFile(filepath.Join(root, "codegen.yaml"), []byte("proto: proto\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runGen(t, "init", "-root", root, "-codegen", codegen); err == nil || !strings.Contains(err.Error(), "初期化済み") {
		t.Errorf("err = %v, want 初期化済みというエラー", err)
	}
}

func TestRun_InitRequiresModule(t *testing.T) {
	root, codegen := newConsumer(t)
	if _, err := runGen(t, "init", "-root", root, "-codegen", codegen); err == nil || !strings.Contains(err.Error(), "-module") {
		t.Errorf("err = %v, want -module が必要というエラー", err)
	}
}

func TestRun_InitRequiresCodegenInsideRoot(t *testing.T) {
	root := t.TempDir()
	if _, err := runGen(t, "init", "-root", root, "-codegen", "../..", "-module", "example.com/x"); err == nil || !strings.Contains(err.Error(), "Submodule") {
		t.Errorf("err = %v, want codegen が -root の中に無いというエラー", err)
	}
}

func TestRun_InitConfigOnly(t *testing.T) {
	// 既存のプロジェクト（api/ に go.mod がある）に、サンプルをコピーせずに入れる
	root, codegen := newConsumer(t)
	if err := os.MkdirAll(filepath.Join(root, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "api", "go.mod"), []byte("module example.com/hub/api\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runGen(t, "init", "-root", root, "-codegen", codegen, "-config-only",
		"-server-out", "api", "-front-out", "web/src/generated", "-client", "@/utils/client", "-title", "Hub API")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	cfg := read(t, filepath.Join(root, "codegen.yaml"))
	for _, want := range []string{"out: api", "out: web/src/generated", "client: '@/utils/client'", "title: Hub API", "proto: proto", "codegen/docs/spec.md"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("codegen.yaml に %q が無い:\n%s", want, cfg)
		}
	}
	for _, sample := range []string{"front", "server", filepath.Join("api", "cmd", "api"), filepath.Join("proto", "article")} {
		if _, err := os.Stat(filepath.Join(root, sample)); err == nil {
			t.Errorf("サンプルの %s をコピーした", sample)
		}
	}
	// proto が無くてもランタイムは生成する
	if got := read(t, filepath.Join(root, "api", "pkg", "util", "errs", "errors.gen.go")); !strings.Contains(got, "package errs") {
		t.Errorf("ランタイムが生成されていない")
	}
	if !strings.Contains(out, "codegen/docs/adopting.md") {
		t.Errorf("導入ガイドを案内していない:\n%s", out)
	}
	if out, err := runGen(t, "check", "-root", root); err != nil {
		t.Errorf("init 直後の check が落ちた: %v\n%s", err, out)
	}
}

func TestRun_InitConfigOnlyWithoutGoMod(t *testing.T) {
	// 出力先に go.mod が無ければ失敗し、codegen.yaml を残さない（直してそのまま再実行できるように）
	root, codegen := newConsumer(t)
	_, err := runGen(t, "init", "-root", root, "-codegen", codegen, "-config-only")
	if err == nil || !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("err = %v, want go.mod が無いというエラー", err)
	}
	if _, err := os.Stat(filepath.Join(root, "codegen.yaml")); err == nil {
		t.Error("失敗したのに codegen.yaml が残っている")
	}
}

func TestRun_InitConfigOnlyRejectsModule(t *testing.T) {
	root, codegen := newConsumer(t)
	_, err := runGen(t, "init", "-root", root, "-codegen", codegen, "-config-only", "-module", "example.com/x")
	if err == nil || !strings.Contains(err.Error(), "-module") {
		t.Errorf("err = %v, want -config-only では -module を使わないというエラー", err)
	}
}
