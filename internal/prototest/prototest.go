// Package prototest はテストで proto ソースからジェネレータを動かすヘルパー。
package prototest

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"google.golang.org/protobuf/compiler/protogen"

	"github.com/zono0013/codegen/internal/openapiload"
	"github.com/zono0013/codegen/internal/protoload"
)

// Files は「パス → proto ソース」をコンパイルする。
func Files(t *testing.T, src map[string]string) []*protogen.File {
	t.Helper()
	files, err := protoload.FromSource(src)
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// Generate は proto ソースに generate を実行し、「出力パス → 内容」を返す。
func Generate(t *testing.T, src map[string]string, generate func([]*protogen.File) (map[string][]byte, error)) map[string]string {
	t.Helper()
	out, err := generate(Files(t, src))
	if err != nil {
		t.Fatal(err)
	}
	strs := map[string]string{}
	for name, content := range out {
		strs[name] = string(content)
	}
	return strs
}

// LoadOpenAPI は生成された（分割された）OpenAPI をメモリ上で $ref を辿って読み込み、妥当性を検証する。
func LoadOpenAPI(t *testing.T, files map[string]string, root string) *openapi3.T {
	t.Helper()
	bs := make(map[string][]byte, len(files))
	for name, src := range files {
		bs[name] = []byte(src)
	}
	doc, err := openapiload.Load(bs, root)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}
