# コントリビューションの手引き

Issue や Pull Request を歓迎します。このファイルは、変更を出す前に知っておいてほしいことをまとめたものです。

- 仕様: [`docs/spec.md`](docs/spec.md)（何が生成されるかの正本）
- 設計: [`docs/architecture.md`](docs/architecture.md)
- 脆弱性の報告: [`SECURITY.md`](SECURITY.md)（公開の Issue には書かない）
- 行動規範: [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md)

## 開発環境

| ツール | 版 |
| --- | --- |
| Go | `go.mod` の `go` ディレクティブ（`GOTOOLCHAIN=auto` なら自動で入る） |
| Node.js | `example/front/.nvmrc` |
| pnpm | `example/front/package.json` の `packageManager` |
| Docker | `example/server` を動かすときだけ |

```bash
git clone https://github.com/zono0013/codegen.git && cd codegen
make test          # ジェネレータのテスト（example/ の生成物が最新か）と、example/server の vet / test
make proto-check   # 生成物が proto・ジェネレータと一致しているか
cd example/front && pnpm install && pnpm typecheck && pnpm lint && pnpm test --run
```

## 変更の進め方

1. **Issue を立てる**（小さな修正なら省略してよい）。何を・なぜ変えるかを書く
2. **ブランチを切る**: `feat/<Issue番号>-<topic>` / `fix/<Issue番号>-<topic>` など（例: `fix/22-unique-constraint-name`）
3. **テストを先に書き、生成物を作り直す**: 振る舞いを変えるときは、テストを先に書いて落ちることを確かめてから実装する。ジェネレータやテンプレート、`example/proto` を変えたら `make proto-gen` で生成物（`*.gen.*`）も更新してコミットする。生成物は手で編集しない。手順は [`docs/spec.md` のテスト方針](docs/spec.md#テスト方針)
4. **Pull Request を出す**: テンプレートに沿って、Summary / Changes / Test plan / Risk を書く

## コミットと Pull Request

- コミットメッセージは [Conventional Commits](https://www.conventionalcommits.org/ja/v1.0.0/)（`feat:` / `fix:` / `docs:` / `refactor:` / `test:` / `ci:` / `chore:`）
- `main` には直接 push できない。Pull Request を通し、**スカッシュマージ**だけを使う（PR のタイトルが `main` のコミットになる）
- マージには CI の必須チェックがすべて通ることが必要（ジェネレータ、example の server / front、Submodule としての E2E、Docker イメージのビルド、zizmor）
- レビューのコメントのスレッドは、解決してからマージする

## 依存の更新

依存の更新は Renovate が Pull Request を出す（[`renovate.json`](renovate.json)）。脆弱性の修正のうち major 以外は、CI が通れば自動でマージされる。手で依存を上げる場合も、GitHub Actions のアクションはタグではなくコミットハッシュで指定する。

## ライセンス

コントリビューションは、このリポジトリのライセンス（[MIT](LICENSE)）で提供されるものとします。
