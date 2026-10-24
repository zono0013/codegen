// Package openapiload は生成された（分割された）OpenAPI をメモリ上で読み込む。
package openapiload

import (
	"context"
	"fmt"
	"net/url"
	"path"

	"github.com/getkin/kin-openapi/openapi3"
)

// Load は「Out からの相対パス → 内容」の files を root から $ref を辿って読み込み、妥当性を検証する。
func Load(files map[string][]byte, root string) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	loader.ReadFromURIFunc = func(_ *openapi3.Loader, uri *url.URL) ([]byte, error) {
		src, ok := files[path.Clean(uri.Path)]
		if !ok {
			return nil, fmt.Errorf("%s: 参照先のファイルが生成されていない", uri.Path)
		}
		return src, nil
	}
	doc, err := loader.LoadFromFile(root)
	if err != nil {
		return nil, fmt.Errorf("OpenAPI を読み込めない: %w", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		return nil, fmt.Errorf("OpenAPI として不正: %w", err)
	}
	return doc, nil
}
