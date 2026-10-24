// example_test は make proto-gen と同じ Targets・同じ処理で example/proto を生成し、
// example/ の生成物が最新で、生成した OpenAPI と Handler のルートが一致していることを検証する。
// proto やジェネレータを変えて make proto-gen を忘れると落ちる。
package codegen_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/zono0013/codegen/internal/check"
	"github.com/zono0013/codegen/internal/config"
	"github.com/zono0013/codegen/internal/pipeline"
	"github.com/zono0013/codegen/internal/protoload"
)

const exampleRoot = "example"

func renderExample(t *testing.T) ([]pipeline.Target, map[string]map[string][]byte) {
	t.Helper()
	cfg, err := config.Load(exampleRoot)
	if err != nil {
		t.Fatal(err)
	}
	protos, err := protoload.FromDir(filepath.Join(exampleRoot, filepath.FromSlash(cfg.Proto)))
	if err != nil {
		t.Fatal(err)
	}
	targets := pipeline.Targets(cfg)
	rendered, err := pipeline.RenderAll(protos, targets)
	if err != nil {
		t.Fatal(err)
	}
	return targets, rendered
}

func TestExample_GeneratedFilesAreUpToDate(t *testing.T) {
	targets, rendered := renderExample(t)
	problems, err := check.Stale(exampleRoot, targets, rendered)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

func TestExample_OpenAPIMatchesServerRoutes(t *testing.T) {
	_, rendered := renderExample(t)
	problems, err := check.Routes(rendered)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		t.Error(p)
	}
}

// codegen.yaml は生成先のプロジェクトにだけ置く。このリポジトリのルートに置くと、
// Submodule として取り込まれたときに codegen/ の中へ生成できてしまう。
func TestRepositoryRootHasNoConfig(t *testing.T) {
	if _, err := os.Stat(config.FileName); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("リポジトリのルートに %s がある（err = %v）", config.FileName, err)
	}
}
