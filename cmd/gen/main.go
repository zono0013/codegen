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
	"bufio"
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
	"github.com/zono0013/codegen/internal/scaffold"
)

// errReported は利用者への報告が済んでいる失敗（check の食い違い、フラグの誤り）を示す。main は終了コードだけ変える。
var errReported = errors.New("already reported")

const usage = `使い方: gen [generate|check|clean|init] [-root DIR] [init のオプション]

  generate  生成する（サブコマンドを省略したときもこれ）
  check     生成物が最新か、OpenAPI と Handler のルートが一致するかを検査する（ファイルには触れない）
  clean     生成物を全部消す
  init      codegen を Submodule として取り込んだプロジェクトに、サンプル（example/）を初回だけコピーし、
            codegen.yaml を書き出して生成する。既にあるファイルは -force が無い限り上書きしない

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
	var initOpts initOptions
	if cmd == "init" {
		fset.StringVar(&initOpts.module, "module", "", "server の Go module（必須。端末から実行したときは省略すると聞く）")
		fset.StringVar(&initOpts.title, "title", "", "OpenAPI の info.title（省略時は「<ルートのディレクトリ名> API」）")
		fset.StringVar(&initOpts.version, "version", "0.1.0", "OpenAPI の info.version")
		fset.StringVar(&initOpts.codegen, "codegen", ".", "codegen（このリポジトリ）の場所。-root の中にあること")
		fset.BoolVar(&initOpts.force, "force", false, "既にあるファイルと codegen.yaml も上書きする")
	}
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
	case "init":
		return runInit(*root, initOpts, stdout)
	default:
		return fmt.Errorf("不明なサブコマンド %q（generate / check / clean / init）", cmd)
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

type initOptions struct {
	module, title, version, codegen string
	force                           bool
}

func runInit(root string, o initOptions, stdout io.Writer) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	absCodegen, err := filepath.Abs(o.codegen)
	if err != nil {
		return err
	}
	codegenDir, err := filepath.Rel(absRoot, absCodegen)
	if err != nil || !filepath.IsLocal(codegenDir) || codegenDir == "." {
		return fmt.Errorf("codegen（%s）が -root（%s）の中に無い。codegen は使う側のリポジトリに Submodule として取り込む", absCodegen, absRoot)
	}
	from := filepath.Join(absCodegen, "example")
	sample, err := config.Load(from)
	if err != nil {
		return err
	}

	opts := scaffold.Options{
		From:       from,
		To:         absRoot,
		CodegenDir: filepath.ToSlash(codegenDir),
		FromModule: sample.Targets.Server.ImportPath,
		Force:      o.force,
	}
	// 初期化済みなら、何かを聞く前に止める
	if err := scaffold.CheckConfigAbsent(opts); err != nil {
		return err
	}

	if o.module == "" && isTerminal(os.Stdin) {
		o.module = prompt(stdout, "server の Go module（例: github.com/you/app/server）: ")
	}
	if o.module == "" {
		return errors.New("-module（server の Go module）が必要")
	}
	if o.title == "" {
		o.title = filepath.Base(absRoot) + " API"
		if isTerminal(os.Stdin) {
			if s := prompt(stdout, fmt.Sprintf("OpenAPI の title [%s]: ", o.title)); s != "" {
				o.title = s
			}
		}
	}

	opts.ToModule = o.module
	// codegen.yaml は最後に書く。コピーが途中で失敗しても「初期化済み」にならず、そのまま再実行できる
	res, err := scaffold.Copy(opts)
	if err != nil {
		return err
	}
	if err := scaffold.WriteConfig(opts, o.title, o.version); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "✓ codegen.yaml を書き出し、サンプルから %d ファイルをコピーした\n", len(res.Written))
	if len(res.Skipped) > 0 {
		fmt.Fprintf(stdout, "  既にあるので触らなかった %d ファイル（上書きするなら -force）:\n", len(res.Skipped))
		for _, f := range res.Skipped {
			fmt.Fprintln(stdout, "   ", f)
		}
	}
	if err := generate(absRoot, stdout); err != nil {
		return err
	}
	fmt.Fprintf(stdout, `
次にやること:
  (cd server && go mod tidy && make test)
  (cd front && pnpm install && pnpm typecheck)
  サンプルの名前（front/package.json の name、front/app/layout.tsx の title、server/Makefile の docker イメージ名）を変える

サンプルのドメイン（Article / User）が不要なら、次を消すか書き換えてから make -C %s proto-gen:
  proto/article/ proto/user/
  server/internal/domain/service/ と server/internal/usecase/ の手書きファイル（テストを含む）
  server/cmd/api/main.go の配線（Usecase / Service の組み立てと di.NewHandlers の引数）
  server/migrations/ の create_articles / create_users と .snapshot.json
  front/app/ と front/features/ の画面
`, filepath.ToSlash(codegenDir))
	return nil
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func prompt(stdout io.Writer, question string) string {
	fmt.Fprint(stdout, question)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line)
}
