// Package scaffold はサンプルアプリ（example/）を、codegen を Submodule として取り込んだプロジェクトへ初回だけコピーする（gen init の実体）。
// コピーした後はそのプロジェクトのものなので、生成物と違って二度と上書きしない。
package scaffold

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/zono0013/codegen/internal/config"
	"github.com/zono0013/codegen/internal/pipeline"
)

// Options はコピーの設定。
type Options struct {
	From       string // サンプルアプリのディレクトリ（codegen の example/）
	To         string // 使う側のプロジェクトのルート
	CodegenDir string // To から見た codegen の場所（スラッシュ区切り。例: "codegen"）
	FromModule string // サンプルの server の Go module（置き換え元）
	ToModule   string // 使う側の server の Go module（置き換え先）
	Force      bool   // 既にあるファイルも上書きする
}

// Result はコピーしたファイルと、既にあったので触らなかったファイル。To からの相対パス。
type Result struct {
	Written []string
	Skipped []string
}

// Copy は From の Git 管理下のファイルを To へコピーする。次のものはコピーしない。
//   - 生成物（1 行目に生成マーカーを持つ）。コピー後の make proto-gen で作り直す
//   - codegen.yaml。使う側の値で別に書き出す
//
// コピーするときに、Go の module を ToModule に、codegen を指す相対パス（example/ から見た ../../ など）を
// CodegenDir を指すものに書き換える。シンボリックリンクの向き先も同じく書き換える。
func Copy(opts Options) (Result, error) {
	var res Result
	files, err := trackedFiles(opts.From)
	if err != nil {
		return res, err
	}
	for _, rel := range files {
		if rel == config.FileName {
			continue
		}
		src := filepath.Join(opts.From, filepath.FromSlash(rel))
		dst := filepath.Join(opts.To, filepath.FromSlash(rel))
		info, err := os.Lstat(src)
		if errors.Is(err, fs.ErrNotExist) {
			continue // 作業ツリーで消されたファイル
		}
		if err != nil {
			return res, err
		}
		depth := strings.Count(rel, "/")

		var write func() error
		if info.Mode()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(src)
			if err != nil {
				return res, err
			}
			target = filepath.FromSlash(rewriteCodegenPaths(filepath.ToSlash(target), depth, opts.CodegenDir))
			write = func() error { return os.Symlink(target, dst) }
		} else {
			content, err := os.ReadFile(src)
			if err != nil {
				return res, err
			}
			if generated(content) {
				continue
			}
			if isText(content) {
				s := rewriteCodegenPaths(string(content), depth, opts.CodegenDir)
				s = replaceModule(s, opts.FromModule, opts.ToModule)
				content = []byte(s)
			}
			write = func() error { return os.WriteFile(dst, content, info.Mode().Perm()) }
		}

		if _, err := os.Lstat(dst); err == nil {
			if !opts.Force {
				res.Skipped = append(res.Skipped, rel)
				continue
			}
			if err := os.Remove(dst); err != nil {
				return res, err
			}
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return res, err
		}
		if err := write(); err != nil {
			return res, err
		}
		res.Written = append(res.Written, rel)
	}
	return res, nil
}

// trackedFiles は dir 配下の Git 管理下のファイルを dir からの相対パス（スラッシュ区切り）で返す。
// node_modules やビルド成果物、.env.local のような手元だけのファイルをコピーしないため、.gitignore に従う。
func trackedFiles(dir string) ([]string, error) {
	// git は PATH から探す。開発者が自分の手元で実行する CLI で、PATH は実行する本人のものなので許容する（SonarQube の go:S4036）
	out, err := exec.Command("git", "-C", dir, "ls-files", "-z", "--", ".").Output() // NOSONAR
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("git ls-files %s: %w: %s（codegen は git clone か Submodule で取り込む）", dir, err, exitErr.Stderr)
		}
		return nil, fmt.Errorf("git ls-files %s: %w", dir, err)
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	return files, nil
}

func generated(content []byte) bool {
	firstLine, _, _ := bytes.Cut(content, []byte("\n"))
	return bytes.Contains(firstLine, []byte(pipeline.Marker))
}

func isText(content []byte) bool {
	return utf8.Valid(content) && !bytes.ContainsRune(content, 0)
}

// rewriteCodegenPaths は、深さ depth（example/ からのディレクトリの段数）にあるファイルの中で
// codegen のルートを指す相対パス（"../" を depth+1 個）を、使う側での codegen の場所を指すものに書き換える。
// それより浅い・深い相対パスはプロジェクト内やプロジェクト外を指すので触らない。
func rewriteCodegenPaths(s string, depth int, codegenDir string) string {
	old := strings.Repeat("../", depth) + ".."
	repl := path.Join(strings.Repeat("../", depth), codegenDir)
	var b strings.Builder
	for {
		i := strings.Index(s, old)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := i + len(old)
		if isCodegenRoot(s, i, end) {
			b.WriteString(s[:i] + repl)
		} else {
			b.WriteString(s[:end])
		}
		s = s[end:]
	}
}

// isCodegenRoot は s[i:end]（"../.." など）が、それ自体で 1 つの相対パスの先頭になっているかを返す。
func isCodegenRoot(s string, i, end int) bool {
	// 直前がパスや名前の一部なら、より深い相対パスの途中か、"1..10" のような別物
	if i > 0 && (s[i-1] == '.' || s[i-1] == '/' || isNameChar(s[i-1])) {
		return false
	}
	if end == len(s) {
		return true
	}
	switch c := s[end]; {
	case c == '/':
		// "/.." が続くなら codegen より上を指している。"/.github" のような隠しディレクトリは codegen の中
		rest := s[end+1:]
		return !(strings.HasPrefix(rest, "..") && (len(rest) == 2 || rest[2] == '/' || !isNameChar(rest[2]) && rest[2] != '.'))
	case c == '.' || isNameChar(c):
		return false // "../..." や "../..foo"
	default:
		return true // 区切り（空白、`、)、"、, など）
	}
}

func isNameChar(c byte) bool {
	return c == '_' || c == '-' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// replaceModule は Go の module のパス from を to に置き換える。from で始まる別の module（from + "less" など）は触らない。
func replaceModule(s, from, to string) string {
	var b strings.Builder
	for {
		i := strings.Index(s, from)
		if i < 0 {
			b.WriteString(s)
			return b.String()
		}
		end := i + len(from)
		if end < len(s) && (isNameChar(s[end]) || s[end] == '.') {
			b.WriteString(s[:end])
		} else {
			b.WriteString(s[:i] + to)
		}
		s = s[end:]
	}
}

var (
	titleLine   = regexp.MustCompile(`(?m)^(\s+title:).*$`)
	versionLine = regexp.MustCompile(`(?m)^(\s+version:).*$`)
)

// CheckConfigAbsent は To に codegen.yaml が無いことを確かめる。Force なら確かめない（使う側の設定を黙って上書きしない）。
func CheckConfigAbsent(opts Options) error {
	dst := filepath.Join(opts.To, config.FileName)
	if _, err := os.Stat(dst); err == nil && !opts.Force {
		return fmt.Errorf("%s が既にある（初期化済み。作り直すなら -force）", dst)
	}
	return nil
}

// WriteConfig は From の codegen.yaml を元に、title / version を差し替えた codegen.yaml を To に書き出す。
func WriteConfig(opts Options, title, version string) error {
	if err := CheckConfigAbsent(opts); err != nil {
		return err
	}
	dst := filepath.Join(opts.To, config.FileName)
	src, err := os.ReadFile(filepath.Join(opts.From, config.FileName))
	if err != nil {
		return err
	}
	s := rewriteCodegenPaths(string(src), 0, opts.CodegenDir)
	// 置き換え文字列の $ は展開されるので、値の $ はエスケープする
	s = titleLine.ReplaceAllString(s, "${1} "+strings.ReplaceAll(strconv.Quote(title), "$", "$$"))
	s = versionLine.ReplaceAllString(s, "${1} "+strings.ReplaceAll(strconv.Quote(version), "$", "$$"))
	if _, err := config.Parse([]byte(s)); err != nil {
		return fmt.Errorf("書き出す codegen.yaml が不正: %w", err)
	}
	return os.WriteFile(dst, []byte(s), 0o644)
}

// WriteNewConfig は cfg を To の codegen.yaml に書き出す。サンプルをコピーせずに既存のプロジェクトへ入れるとき（init -config-only）に使う。
func WriteNewConfig(opts Options, cfg config.Config) error {
	if err := CheckConfigAbsent(opts); err != nil {
		return err
	}
	var b bytes.Buffer
	b.WriteString("# codegen の設定。パスはすべてこのファイルがあるディレクトリからの相対（仕様は " + opts.CodegenDir + "/docs/spec.md）。\n")
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	if _, err := config.Parse(b.Bytes()); err != nil {
		return fmt.Errorf("書き出す codegen.yaml が不正: %w", err)
	}
	return os.WriteFile(filepath.Join(opts.To, config.FileName), b.Bytes(), 0o644)
}
