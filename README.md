# codegen

proto を唯一のソースにして **Go バックエンド + Next.js フロントエンド + OpenAPI** を生成するジェネレータ。使う側のリポジトリに Git Submodule（`codegen/`）として取り込んで使う。

`proto/**/*.proto` を書いて `make -C codegen proto-gen` を叩くと、Entity / Repository / Mock / Postgres 実装 / DTO / Usecase interface / REST Handler / DI 配線（Go 側）と、Zod スキーマ / 型付き API クライアント（TS 側）、OpenAPI 仕様が生成される。**手書きが残るのは、main の配線、Service と Usecase の実装、FE の画面と apiClient。**

## 使い方（Submodule として取り込む）

```bash
# 1. 使う側のリポジトリに codegen/ として取り込む
git submodule add https://github.com/zono0013/codegen codegen

# 2. ルートに codegen.yaml を書く（下の例をコピー）

# 3. server の Go module を作る（生成物の import path は go.mod から決まる）
mkdir -p server && (cd server && go mod init github.com/you/app/server)

# 4. proto/ に proto を書いて生成し、生成物が使うライブラリを go.mod に入れる
make -C codegen proto-gen
(cd server && go mod tidy)
```

```yaml
# codegen.yaml（パスはすべてこのファイルからの相対。仕様は codegen/docs/spec.md）
proto: proto
targets:
  server:
    out: server
  front:
    out: front/generate
    client: "@/shared/api/apiClient"   # 生成した API 関数が import する apiClient（front 側で用意する）
  openapi:
    out: docs/openapi
    title: My API
    version: 0.1.0
```

clone し直すときは `git clone --recursive`、または clone の後に `git submodule update --init`。

### 生成物の外で用意するもの

生成物は次のものを前提にしている。使う側で書く。書き方は `example/` が見本になる。example は codegen を `../..` などの相対パスで参照しているので、写すときはそのパスを直す。

| 何 | 見本 |
| --- | --- |
| server の go.mod（`go` は 1.22 以上。生成する Handler は Go 1.22 の ServeMux のパターンで登録する） | `example/server/go.mod` |
| `main`：DB を開いて `tx.Init(db)`、Repository → Service → Usecase → `di.NewHandlers` と組み立て、`Register(mux)` で ServeMux に登録する | `example/server/cmd/api/main.go` |
| Service と Usecase の実装（生成された Usecase interface を実装する） | `example/server/internal/domain/service/`、`example/server/internal/usecase/` |
| マイグレーションの適用（`go run ./cmd/migration-gen` が SQL を作る。適用は golang-migrate など） | `example/server/Makefile` |
| front の apiClient（`codegen.yaml` の `targets.front.client` で指定。axios のインスタンスがそのまま使える。期待する形は [`docs/spec.md`](docs/spec.md) の「ランタイム」）と `zod` | `example/front/shared/api/apiClient.ts` |

リポジトリのルート以外（`v2/` など）を生成先にするときは、そのディレクトリに `codegen.yaml` と `proto/` を置き（`codegen.yaml` からは自分のディレクトリの外を指せない）、`make -C codegen proto-gen ROOT=$PWD/v2` で生成する。`ROOT` は絶対パスで書く（相対パスは `codegen/` から解決される）。

### 使う側のリポジトリの形

```
my-app/
├── codegen/          ← Submodule（このリポジトリ）。中には書き込まない
├── codegen.yaml      ← 生成の設定（手で書く）
├── proto/            ← 手書き。唯一のソース
├── server/           ← 手書き（main、Service / Usecase 実装）+ 毎回生成（**/*.gen.go）
├── front/            ← 手書き（画面、apiClient）+ 毎回生成（generate/**/*.gen.ts）
└── docs/openapi/     ← 毎回生成（**/*.gen.yaml）
```

`*.gen.*` は `make proto-gen` のたびに書き出す。手で編集しない。Submodule を上げて `make -C codegen proto-gen` すると、codegen の更新に追従する。

**何が生成されるか**（ファイルごとの役割、アノテーション、REST の振る舞い、OpenAPI の構成、ランタイム）は [`docs/spec.md`](docs/spec.md) にまとめてある（正本）。`example/` の `*.gen.*` が実際の出力例。

## 開発サイクル

```
proto を編集
   ↓
make -C codegen proto-gen          # BE / FE / OpenAPI の生成物を作り直す
   ↓
cd server && go run ./cmd/migration-gen   # entity の差分から up SQL を生成
   ↓
migrate -path migrations -database "$DATABASE_URL" up   # DB に適用（golang-migrate）
   ↓
コンパイルエラーになった手書き層（Service / Usecase 実装 / 画面）を直す
```

proto を変えると生成物が変わり、それを使う手書き層がコンパイルエラーになる。**型で影響範囲が出る**のがこの構成の要。

## コマンド

使う側では `make -C codegen <target>`。生成先は、Submodule として取り込まれていれば親リポジトリのルート、このリポジトリ単体なら `example/` になる（`ROOT=<絶対パス>` で上書きできる）。

| コマンド | 内容 |
| --- | --- |
| `make proto-gen` | BE + FE + OpenAPI を生成（古い生成物は削除） |
| `make proto-check` | 生成物が最新か、OpenAPI が妥当かつサーバーの実ルートと一致するかを検査（ファイルには触れない） |
| `make proto-clean` | 生成物を削除 |
| `make test` | ジェネレータのテスト（このリポジトリの開発用） |

## スキーマ適用

`AutoMigrate` は **起動時に呼ばない**。`go run ./cmd/migration-gen`（example では `make migration-gen`）が entity の gorm タグから差分 SQL を吐き、`golang-migrate` で適用する一本道にしてある（GORM と生成する SQL で制約の命名が違い、混ぜると制約の状態が食い違うため）。生成される `AutoMigrate<Entity>` ヘルパーは integration test 用。

## このリポジトリの構成

```
codegen/
├── cmd/gen/                # CLI（generate / check / clean）
├── internal/               # ジェネレータ本体（Go / TS / OpenAPI）、テンプレート、ランタイムの元
├── docs/
│   ├── spec.md             # 自動生成の仕様（正本）
│   └── architecture.md     # 設計（層の構成・考え方・向き不向き・DB を変えたいとき）
├── example/                # サンプルアプリ（記事投稿: Article + User）。生成結果の見本と CI の検証用
│   ├── codegen.yaml
│   ├── proto/
│   ├── server/             # Go (DDD + クリーンアーキテクチャ + REST)
│   ├── front/              # Next.js App Router (FSD)
│   └── docs/openapi/
├── .github/workflows/      # CI（ジェネレータ / example / Docker イメージ / Submodule としての E2E）と Security（govulncheck / pnpm audit / zizmor。毎日も実行）
└── Makefile
```

### サンプルを動かす

```bash
make proto-gen                                     # example/ に生成
cd example/server && make install-tools            # golang-migrate / migration-gen
cp .env.example .env.local && docker compose up -d db && make migrate-up
make run                                           # API（:8080）
cd ../front && pnpm install && pnpm dev            # 別ターミナルで FE（:3000）
```

## ドキュメント

| ファイル | 内容 |
| --- | --- |
| [`docs/spec.md`](docs/spec.md) | **自動生成の仕様（設定ファイル / コマンド / ランタイム / アノテーション / REST の振る舞い / OpenAPI / テスト方針）** |
| [`docs/architecture.md`](docs/architecture.md) | 設計（層の構成・考え方・向き不向き・DB を変えたいとき） |
| [`example/server/CLAUDE.md`](example/server/CLAUDE.md) | example の BE の前提・規約 |
| [`example/front/CLAUDE.md`](example/front/CLAUDE.md) | example の FE の前提・規約 |
| [`example/front/docs/`](example/front/docs/) | example の FSD 構成 / コーディング規約 / デザイントークン / テスト方針 |

## コントリビューション

Issue や Pull Request を歓迎します。[`CONTRIBUTING.md`](CONTRIBUTING.md) を読んでください。脆弱性は [`SECURITY.md`](SECURITY.md) の手順で非公開に報告してください。

## ライセンス

[MIT](LICENSE)
