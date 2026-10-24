# codegen

proto を唯一のソースにして **Go バックエンド + Next.js フロントエンド + OpenAPI** を生成するジェネレータ。使う側のリポジトリに Git Submodule（`codegen/`）として取り込んで使う。

`proto/**/*.proto` を書いて `make -C codegen proto-gen` を叩くと、Entity / Repository / Mock / Postgres 実装 / DTO / Usecase interface / REST Handler / DI 配線（Go 側）と、Zod スキーマ / 型付き API クライアント（TS 側）、OpenAPI 仕様が生成される。**手書きが残るのは Service と Usecase 実装、そして FE の画面だけ。**

## 使い方（Submodule として取り込む）

```bash
# 1. 使う側のリポジトリに codegen/ として取り込む
git submodule add https://github.com/zono0013/codegen codegen

# 2. 初回だけ: サンプル（example/）をコピーし、codegen.yaml を作って生成する
make -C codegen codegen-init MODULE=github.com/you/app/server TITLE="App API"

# 3. あとは proto を書いて生成する
make -C codegen proto-gen
```

clone し直すときは `git clone --recursive`、または clone の後に `git submodule update --init`。

### 使う側のリポジトリの形

```
my-app/
├── codegen/          ← Submodule（このリポジトリ）。中には書き込まない
├── codegen.yaml      ← 生成の設定（codegen-init が作る）
├── proto/            ← 手書き。唯一のソース
├── server/           ← 初回コピー（手書き層）+ 毎回生成（**/*.gen.go）
├── front/            ← 初回コピー（画面）+ 毎回生成（generate/**/*.gen.ts）
└── docs/openapi/     ← 毎回生成（**/*.gen.yaml）
```

ファイルは次の 3 種類に分かれる。

| 種類 | 何か | codegen を更新したとき |
| --- | --- | --- |
| **毎回生成** | `*.gen.*`。`make proto-gen` のたびに書き出す。手で編集しない | Submodule を上げて `make -C codegen proto-gen` すると追従する |
| **初回コピー** | `codegen-init` が `example/` からコピーしたもの（`main.go`、Service / Usecase 実装、画面、Makefile、Dockerfile など）。その後は使う側のもの | 上書きしない |
| **参照** | `codegen/` の中をそのまま読むもの（`docs/spec.md`、`docs/architecture.md`） | Submodule を上げれば変わる |

**何が生成されるか**（ファイルごとの役割、アノテーション、REST の振る舞い、OpenAPI の構成、ランタイム）は [`docs/spec.md`](docs/spec.md) にまとめてある（正本）。`example/` の `*.gen.*` が実際の出力例。

## 開発サイクル

```
proto を編集
   ↓
make -C codegen proto-gen          # BE / FE / OpenAPI の生成物を作り直す
   ↓
cd server && make migration-gen    # entity の差分から up SQL を生成
   ↓
make migrate-up                    # DB に適用
   ↓
コンパイルエラーになった手書き層（Service / Usecase 実装 / 画面）を直す
```

proto を変えると生成物が変わり、それを使う手書き層がコンパイルエラーになる。**型で影響範囲が出る**のがこの構成の要。

## コマンド

使う側では `make -C codegen <target>`。生成先は、Submodule として取り込まれていれば親リポジトリのルート、このリポジトリ単体なら `example/` になる（`ROOT=<dir>` で上書きできる）。

| コマンド | 内容 |
| --- | --- |
| `make codegen-init MODULE=<module> [TITLE=<API 名>]` | 初回だけ。サンプルをコピーし、`codegen.yaml` を作って生成する。既にあるファイルは上書きしない。`codegen.yaml` があると「初期化済み」で止まる（作り直すなら `go run ./cmd/gen init -force ...`） |
| `make proto-gen` | BE + FE + OpenAPI を生成（古い生成物は削除） |
| `make proto-check` | 生成物が最新か、OpenAPI が妥当かつサーバーの実ルートと一致するかを検査（ファイルには触れない） |
| `make proto-clean` | 生成物を削除 |
| `make test` | ジェネレータのテスト（このリポジトリの開発用） |

## スキーマ適用

`AutoMigrate` は **起動時に呼ばない**。`make migration-gen` が entity の gorm タグから差分 SQL を吐き、`golang-migrate` で適用する一本道にしてある（GORM が期待する制約名と生成 SQL の制約名が食い違い、既存スキーマを壊すため）。生成される `AutoMigrate<Entity>` ヘルパーは integration test 用。

## このリポジトリの構成

```
codegen/
├── cmd/gen/                # CLI（generate / check / clean / init）
├── internal/               # ジェネレータ本体（Go / TS / OpenAPI）、テンプレート、ランタイムの元
├── docs/
│   ├── spec.md             # 自動生成の仕様（正本）
│   └── architecture.md     # 設計（層の構成・考え方・向き不向き・DB を変えたいとき）
├── example/                # サンプルアプリ（記事投稿: Article + User）。codegen-init のコピー元
│   ├── codegen.yaml
│   ├── proto/
│   ├── server/             # Go (DDD + クリーンアーキテクチャ + REST)
│   ├── front/              # Next.js App Router (FSD)
│   └── docs/openapi/
├── .github/workflows/      # CI（ジェネレータ / example / Submodule としての E2E）
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
| [`example/server/CLAUDE.md`](example/server/CLAUDE.md) | BE の前提・規約（初回コピーされる） |
| [`example/front/CLAUDE.md`](example/front/CLAUDE.md) | FE の前提・規約（同上） |
| [`example/front/docs/`](example/front/docs/) | FSD 構成 / コーディング規約 / デザイントークン / テスト方針（同上） |

## ライセンス

[MIT](LICENSE)
