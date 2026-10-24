// Package config は生成先のプロジェクトのルートに置く codegen.yaml を読む。
// 何をどこへ生成するかはプロジェクトごとに違うので、ジェネレータには持たせずこのファイルで受け取る。
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// FileName は生成先のプロジェクトのルートに置く設定ファイルの名前。
const FileName = "codegen.yaml"

// Config は codegen.yaml の内容。パスはすべて codegen.yaml があるディレクトリからの相対（スラッシュ区切り）。
type Config struct {
	Proto   string  `yaml:"proto"` // proto/**/*.proto を置くディレクトリ
	Targets Targets `yaml:"targets"`
}

type Targets struct {
	Server  Server  `yaml:"server"`
	Front   Front   `yaml:"front"`
	OpenAPI OpenAPI `yaml:"openapi"`
}

type Server struct {
	Out string `yaml:"out"`
	// ImportPath は Out の Go の import path。codegen.yaml には書かず、Load が go.mod から求める（二重に持たないため）。
	ImportPath string `yaml:"-"`
}

type Front struct {
	Out string `yaml:"out"`
	// Client は生成した API 関数が import する apiClient の指定子。apiClient は認証などプロジェクトの方針を含むので生成しない。
	Client string `yaml:"client"`
}

type OpenAPI struct {
	Out     string `yaml:"out"`
	Title   string `yaml:"title"`
	Version string `yaml:"version"`
}

// Load は root/codegen.yaml を読む。無い場合は fs.ErrNotExist を包んだエラーを返す。
func Load(root string) (Config, error) {
	p := filepath.Join(root, FileName)
	data, err := os.ReadFile(p)
	if err != nil {
		return Config{}, fmt.Errorf("%s を読めない（生成先のプロジェクトのルートに置く。書式は codegen の docs/spec.md）: %w", p, err)
	}
	cfg, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", p, err)
	}
	for _, o := range []struct{ key, out string }{
		{"targets.server.out", cfg.Targets.Server.Out},
		{"targets.front.out", cfg.Targets.Front.Out},
		{"targets.openapi.out", cfg.Targets.OpenAPI.Out},
	} {
		if err := outsideOtherRepositories(root, o.out); err != nil {
			return Config{}, fmt.Errorf("%s: %s: %w", p, o.key, err)
		}
	}
	if cfg.Targets.Server.ImportPath, err = importPath(root, cfg.Targets.Server.Out); err != nil {
		return Config{}, fmt.Errorf("%s: targets.server.out: %w", p, err)
	}
	return cfg, nil
}

// outsideOtherRepositories は out から codegen.yaml のあるディレクトリまで（そのものは含まない）に、
// 別の Git リポジトリ（Submodule を含む。Submodule のチェックアウトは .git をファイルとして持つ）が無いことを確かめる。
// codegen を Submodule として取り込んだ側が、誤って codegen/ の中を出力先にしないため。
func outsideOtherRepositories(root, out string) error {
	for dir := out; dir != "."; dir = path.Dir(dir) {
		_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(dir), ".git"))
		if err == nil {
			return fmt.Errorf("%s は別の Git リポジトリ（Submodule など）なので、その中には生成しない", dir)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	return nil
}

// importPath は root/out の Go の import path を、out から root までを遡って最初に見つけた go.mod から求める。
// root より上は見ない（別プロジェクトの go.mod かもしれないため）。
func importPath(root, out string) (string, error) {
	out = path.Clean(out)
	for dir := out; ; dir = path.Dir(dir) {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dir), "go.mod"))
		if err == nil {
			mod, err := modulePath(data)
			if err != nil {
				return "", fmt.Errorf("%s/go.mod: %w", dir, err)
			}
			if dir == out {
				return mod, nil
			}
			return path.Join(mod, strings.TrimPrefix(out, dir+"/")), nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if dir == "." {
			return "", fmt.Errorf("%s から codegen.yaml のあるディレクトリまでに go.mod が無い", out)
		}
	}
}

// modulePath は go.mod の module ディレクティブの値を返す。Go が受け付ける書き方（行末コメント、引用符、
// "module ( ... )" のブロック）を読み、Go が拒む書き方（値が無い、2 回ある）はエラーにする。
func modulePath(gomod []byte) (string, error) {
	var found []string
	inBlock := false
	for _, line := range strings.Split(string(gomod), "\n") {
		line, _, _ = strings.Cut(line, "//")
		fields := strings.Fields(line)
		switch {
		case inBlock && len(fields) == 1 && fields[0] == ")":
			inBlock = false
		case inBlock && len(fields) > 0:
			found = append(found, strings.Join(fields, " "))
		case len(fields) >= 1 && fields[0] == "module":
			if len(fields) == 2 && fields[1] == "(" {
				inBlock = true
				continue
			}
			found = append(found, strings.Join(fields[1:], " "))
		}
	}
	switch {
	case inBlock:
		return "", errors.New("module ( が閉じていない")
	case len(found) == 0:
		return "", errors.New("module ディレクティブが無い")
	case len(found) > 1:
		return "", errors.New("module ディレクティブが 2 つ以上ある")
	}
	mod := found[0]
	if unquoted, err := strconv.Unquote(mod); err == nil && strings.HasPrefix(mod, `"`) {
		mod = unquoted
	}
	if mod == "" || strings.ContainsAny(mod, " \t`\"") {
		return "", fmt.Errorf("module の値 %q が読めない", found[0])
	}
	return mod, nil
}

// Parse は codegen.yaml の内容を読み、検証する。パスは正規化して返す（"./server/" → "server"）。
// 未知のキーはタイプミスとみなしてエラーにする。
func Parse(data []byte) (Config, error) {
	var cfg Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	// 空ファイル（コメントだけを含む）は EOF になる。「必須」の検証で何が足りないかを示す
	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return Config{}, err
	}
	return cfg, cfg.normalize()
}

// normalize はパスを正規化しながら検証する。
func (c *Config) normalize() error {
	var errs []error
	dirs := []struct {
		key   string
		value *string
	}{
		{"proto", &c.Proto},
		{"targets.server.out", &c.Targets.Server.Out},
		{"targets.front.out", &c.Targets.Front.Out},
		{"targets.openapi.out", &c.Targets.OpenAPI.Out},
	}
	for _, d := range dirs {
		clean, err := cleanDir(*d.value)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", d.key, err))
			continue
		}
		*d.value = clean
	}
	for _, f := range []struct{ key, value string }{
		{"targets.front.client", c.Targets.Front.Client},
		{"targets.openapi.title", c.Targets.OpenAPI.Title},
		{"targets.openapi.version", c.Targets.OpenAPI.Version},
	} {
		if strings.TrimSpace(f.value) == "" {
			errs = append(errs, fmt.Errorf("%s: 必須", f.key))
		}
	}
	// client は生成する TypeScript の import 文に '...' でそのまま埋め込む
	if strings.ContainsAny(c.Targets.Front.Client, "'\"\\\n\r") {
		errs = append(errs, fmt.Errorf("targets.front.client: %q: 引用符・バックスラッシュ・改行は使えない", c.Targets.Front.Client))
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	// 出力先が入れ子だと、片方の古い生成物の削除がもう片方の生成物を消しうる
	outs := dirs[1:]
	for i, a := range outs {
		for _, b := range outs[i+1:] {
			if overlaps(*a.value, *b.value) {
				errs = append(errs, fmt.Errorf("%s (%s) と %s (%s) の出力先が重なっている", a.key, *a.value, b.key, *b.value))
			}
		}
	}
	return errors.Join(errs...)
}

// cleanDir は dir が codegen.yaml のあるディレクトリの中（そのものは含まない）を指す相対パスかを検証し、正規化して返す。
// ディレクトリそのもの（"." や "server/.."）を許すと、古い生成物の削除がプロジェクト全体（Submodule を含む）を走査してしまう。
func cleanDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", errors.New("必須")
	}
	if strings.Contains(dir, `\`) {
		return "", errors.New("区切りは / で書く")
	}
	// OS に依存しないよう、スラッシュ区切りのまま path で判定する
	clean := path.Clean(dir)
	if path.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return "", fmt.Errorf("%q: codegen.yaml のあるディレクトリの中（そのものは含まない）を指す相対パスで書く", dir)
	}
	return clean, nil
}

// overlaps は正規化済みの a と b が同じか、片方がもう片方の中にあるかを返す。
func overlaps(a, b string) bool {
	a, b = a+"/", b+"/"
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}
