// Package check は生成先のプロジェクトが proto・ジェネレータと食い違っていないかを検査する（gen check の実体）。
package check

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/zono0013/codegen/internal/openapigen"
	"github.com/zono0013/codegen/internal/openapiload"
	"github.com/zono0013/codegen/internal/pipeline"
)

// HandlersFile はルートを登録する生成物の、server Target の Out からのパス。
const HandlersFile = "internal/di/handlers.gen.go"

// Problem は検査で見つかった食い違い 1 件。
type Problem struct {
	Target  string
	File    string // Target の Out からの相対パス。ファイルに紐づかない問題では空
	Message string
}

func (p Problem) String() string {
	if p.File == "" {
		return p.Target + ": " + p.Message
	}
	return p.Target + ": " + p.File + ": " + p.Message
}

// Stale は root 配下の生成物が rendered（make proto-gen が書くはずの内容）と一致しない箇所を返す。
// proto やジェネレータを変えて make proto-gen を忘れると見つかる。
func Stale(root string, targets []pipeline.Target, rendered map[string]map[string][]byte) ([]Problem, error) {
	var problems []Problem
	for _, t := range targets {
		files := rendered[t.Name]
		plan, err := pipeline.Plan(root, t, files)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.Name, err)
		}
		for _, name := range plan.Written {
			committed, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(t.Out), filepath.FromSlash(name)))
			if errors.Is(err, fs.ErrNotExist) {
				problems = append(problems, Problem{Target: t.Name, File: name, Message: "生成物が無い（make proto-gen 未実行?）"})
				continue
			}
			if err != nil {
				return nil, fmt.Errorf("%s: %s: %w", t.Name, name, err)
			}
			problems = append(problems, Problem{Target: t.Name, File: name, Message: "生成物が最新でない（make proto-gen 未実行?）\n" + firstDiff(string(committed), string(files[name]))})
		}
		for _, name := range plan.Deleted {
			problems = append(problems, Problem{Target: t.Name, File: name, Message: "生成されない古い生成物が残っている"})
		}
	}
	return problems, nil
}

// firstDiff は最初に食い違った行だけを示す（生成物全体を出すと読めないため）。
func firstDiff(committed, generated string) string {
	c, g := strings.Split(committed, "\n"), strings.Split(generated, "\n")
	for i := 0; i < len(c) || i < len(g); i++ {
		var cl, gl string
		if i < len(c) {
			cl = c[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if cl != gl {
			return "line " + strconv.Itoa(i+1) + ":\n  committed: " + cl + "\n  generated: " + gl
		}
	}
	return "(末尾の改行のみ異なる)"
}

var routeRe = regexp.MustCompile(`mux\.HandleFunc\("(\w+) ([^"]+)", h\.\w+\.(\w+)\)`)

// Routes は生成した OpenAPI が妥当で、生成した Handler のルートとメソッド・パス・operationId が 1:1 で一致しているかを検査する。
func Routes(rendered map[string]map[string][]byte) ([]Problem, error) {
	doc, err := openapiload.Load(rendered["openapi"], openapigen.RootFile)
	if err != nil {
		return nil, err
	}
	spec := map[string]string{} // "METHOD /path" → operationId
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			spec[method+" "+path] = op.OperationID
		}
	}

	matches := routeRe.FindAllStringSubmatch(string(rendered["server"][HandlersFile]), -1)
	// service が無ければルートも仕様の path も無い。仕様に path があるのに 1 件も読めないときだけ書式の変化を疑う
	if len(matches) == 0 && len(spec) > 0 {
		return nil, fmt.Errorf("%s からルートを 1 件も読めない（登録の書式が変わった?）", HandlersFile)
	}
	server := map[string]string{} // "METHOD /path" → Handler メソッド名
	for _, m := range matches {
		server[m[1]+" "+m[2]] = m[3]
	}

	var problems []Problem
	if len(server) != len(matches) {
		problems = append(problems, Problem{Target: "server", File: HandlersFile, Message: "同じルートが重複登録されている"})
	}
	for _, route := range slices.Sorted(maps.Keys(server)) {
		if op, ok := spec[route]; !ok {
			problems = append(problems, Problem{Target: "server", File: HandlersFile, Message: route + ": サーバーにあるが仕様に無い"})
		} else if op != server[route] {
			problems = append(problems, Problem{Target: "server", File: HandlersFile, Message: fmt.Sprintf("%s: operationId %q が Handler %q と一致しない", route, op, server[route])})
		}
	}
	for _, route := range slices.Sorted(maps.Keys(spec)) {
		if _, ok := server[route]; !ok {
			problems = append(problems, Problem{Target: "openapi", Message: route + ": 仕様にあるがサーバーに無い"})
		}
	}
	return problems, nil
}
