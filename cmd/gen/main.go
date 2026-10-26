// gen は proto/ から server / front / docs/openapi の生成物を作る（make proto-gen の実体）。
//
// DIR は生成先のプロジェクトのルート。DIR/codegen.yaml の設定に従う。
//
//	gen [-root DIR]         生成する（generate の省略形）
//	gen generate [-root DIR]
//	gen check [-root DIR]   生成物が最新か、OpenAPI と Handler のルートが一致するかを検査する（ファイルには触れない）
//	gen clean [-root DIR]   生成物を全部消す
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zono0013/codegen/internal/check"
	"github.com/zono0013/codegen/internal/config"
	"github.com/zono0013/codegen/internal/pipeline"
	"github.com/zono0013/codegen/internal/protoload"
)

// errReported は利用者への報告が済んでいる失敗（check の食い違い、フラグの誤り）を示す。main は終了コードだけ変える。
var errReported = errors.New("already reported")

const usage = `使い方: gen [generate|check|clean] [-root DIR]

  generate  生成する（サブコマンドを省略したときもこれ）
  check     生成物が最新か、OpenAPI と Handler のルートが一致するかを検査する（ファイルには触れない）
  clean     生成物を全部消す

`

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		if !errors.Is(err, errReported) {
			fmt.Fprintln(os.Stderr, "gen:", err)
		}
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	cmd := "generate"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	fset := flag.NewFlagSet("gen "+cmd, flag.ContinueOnError)
	root := fset.String("root", ".", "生成先のプロジェクトのルート（codegen.yaml を置くディレクトリ）")
	fset.Usage = func() {
		fmt.Fprint(fset.Output(), usage)
		fset.PrintDefaults()
	}
	if err := fset.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errReported // FlagSet がエラーと使い方を出力済み
	}
	if fset.NArg() > 0 {
		return fmt.Errorf("余分な引数: %v（サブコマンドは -root より前に書く）", fset.Args())
	}

	switch cmd {
	case "generate":
		return generate(*root, stdout)
	case "check":
		return runCheck(*root, stdout)
	case "clean":
		return clean(*root, stdout)
	default:
		return fmt.Errorf("不明なサブコマンド %q（generate / check / clean）", cmd)
	}
}

func render(root string) ([]pipeline.Target, map[string]map[string][]byte, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return nil, nil, err
	}
	protos, err := protoload.FromDir(filepath.Join(root, filepath.FromSlash(cfg.Proto)))
	if err != nil {
		return nil, nil, err
	}
	targets := pipeline.Targets(cfg)
	rendered, err := pipeline.RenderAll(protos, targets)
	return targets, rendered, err
}

func generate(root string, stdout io.Writer) error {
	// 全 Target を生成し終えてから書き出す（途中で失敗しても生成物を中途半端に壊さない）
	targets, rendered, err := render(root)
	if err != nil {
		return err
	}
	for _, t := range targets {
		res, err := pipeline.Write(root, t, rendered[t.Name])
		if err != nil {
			return fmt.Errorf("%s: %w", t.Name, err)
		}
		fmt.Fprintf(stdout, "✓ %-8s %s: %d files (%d updated, %d deleted)\n", t.Name, t.Out, len(rendered[t.Name]), len(res.Written), len(res.Deleted))
	}
	return nil
}

func runCheck(root string, stdout io.Writer) error {
	targets, rendered, err := render(root)
	if err != nil {
		return err
	}
	stale, err := check.Stale(root, targets, rendered)
	if err != nil {
		return err
	}
	// Routes が検査できなくても、生成物の古さは先に報告する
	routes, routesErr := check.Routes(rendered)
	problems := append(stale, routes...)
	for _, p := range problems {
		fmt.Fprintln(stdout, "✗", p)
	}
	if routesErr != nil {
		return routesErr
	}
	if len(problems) > 0 {
		fmt.Fprintf(stdout, "%d 件の食い違い\n", len(problems))
		return errReported
	}
	fmt.Fprintln(stdout, "✓ 生成物は最新で、OpenAPI と Handler のルートは一致している")
	return nil
}

func clean(root string, stdout io.Writer) error {
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	// 何も生成しない扱いで Write すると、マーカーを持つ生成物だけが「古い生成物」として消える
	for _, t := range pipeline.Targets(cfg) {
		res, err := pipeline.Write(root, t, nil)
		if err != nil {
			return fmt.Errorf("%s: %w", t.Name, err)
		}
		fmt.Fprintf(stdout, "✓ %-8s %s: %d deleted\n", t.Name, t.Out, len(res.Deleted))
	}
	return nil
}
