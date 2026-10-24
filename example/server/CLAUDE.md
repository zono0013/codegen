# CLAUDE.md

Go + DDD + クリーンアーキテクチャ + REST のバックエンドプロジェクト。

## 技術スタック

- **言語**: Go 1.25
- **API スタイル**: REST(JSON over HTTP)。URL は proto の rpc に付けた `@http METHOD /path` アノテーションで宣言する
- **Proto**: Protocol Buffers。codegen(`../../`)の `make proto-gen`(buf は使わない)でドメイン層〜ハンドラまで生成
- **コード生成プラグイン**:
  - `../../`(codegen。独自の Go モジュールで、server / front / OpenAPI の生成を一括で担い、proto の解釈を共有する。**生成の仕様は `../../docs/spec.md` が正本**) — proto の `@entity` / `@http` マーカーから以下を生成:
    - Entity / Repository interface / Mock / Postgres Repository 実装(エンティティ由来)
    - Usecase interface + Input 型、REST Handler 実装(`net/http`)、DI 用 `Handlers` struct(service 由来)
  - **protoc-gen-go / protoc-gen-connect-go は採用しない**(REST なので proto メッセージ型は不要)
- **HTTP ルーティング**: 標準 `net/http` + `http.ServeMux`(Go 1.22+ の `"METHOD /path/{param}"` パターン)
- **データストア**: PostgreSQL（GORM `gorm.io/gorm` + `gorm.io/driver/postgres`、生成物の単一実装）
- **テスト方針**: ユニットテストは生成 Mock を使う。結合テストは docker-compose / testcontainers の Postgres に接続
- **ビルド**: make + go

> 別 DB（MySQL / Redis / MongoDB 等）へ移管する手順は `../../docs/patterns/infra-swap.md` を参照。この CLAUDE.md の「データストア」欄を更新すると、以降のエージェント（Claude Code など）の判断に反映される。

## アーキテクチャ

- **パターン**: DDD(ドメイン駆動設計) + クリーンアーキテクチャ
- **レイヤー構成**:
  ```
  Proto (.proto)
   └→ make proto-gen (../../internal/gogen) で以下を生成:
      internal/domain/entity/*.gen.go                          (Entity + GORM タグ)
      internal/domain/entity/registry.gen.go                   (var All = []any{...} migration-gen 用)
      internal/domain/repository/*_repository.gen.go           (interface)
      internal/domain/repository/mock/mock_*_repository.gen.go (テスト用スタブ)
      internal/infra/repository/*_postgres_repository.gen.go   (Postgres + GORM)
      internal/dto/*.gen.go                                    (DTO + entity → DTO 変換関数)
      internal/usecase/*_usecase_interface.gen.go              (Usecase interface + Input 型)
      internal/handler/*_handler.gen.go                        (REST Handler、net/http ベース)
      internal/di/handlers.gen.go                              (Handlers struct + Register(mux))

  Entity (生成)
   └→ migration-gen (cmd/migration-gen) で以下を生成:
      migrations/<YYYYMMDDHHMMSS>_<table>.up.sql               (Postgres up migration、1 entity につき 1 file)
      migrations/.snapshot.json                                (前回適用 schema、git commit する)
                            ↓
  Handler (生成) ──json.Encode──→ DTO (生成)
       ↓ 呼び出し                  ↑ 変換(usecase 内で dto.From*)
  Usecase interface (生成)        Entity (生成)
       ↑ 実装                       ↑ GORM が直接読み書き
  <Name>UsecaseImpl (手書き) → Service (手書き) → Repository (生成)
                                                     ↑ 実装
                                          Postgres Repository (生成)
  ```
- **依存方向**: 常に内側(Entity)に向かう。Entity は他層を import しない。
- **DI**: コンストラクタ注入(`cmd/api/main.go` でワイヤリング)
- **DTO 境界**: クライアントへ返す JSON 表現は **DTO 層が一元管理** する。Usecase が `*entity.X` を `*dto.XDTO` に変換して返し、Handler は `json.Encode(result)` するだけ。Entity 側の json タグは持たず、`gorm:` タグのみ。
- **Entity = ORM モデル**: 生成 Entity は GORM の `column / primaryKey / uniqueIndex / not null` タグを所有し、`TableName()` も自動付与される。`*entity.X` を直接 `db.Create / Find` に渡せる(`HydrateX` のような中間モデルを噛ませない)。
- **可視性境界**: ドメイン〜インフラ〜DI〜Handler はすべて `internal/` 配下に置き、外部モジュールから import 不可にする。`pkg/util/` だけは横断的ユーティリティ(env / logger / errs / tx)として `pkg/` に残し、外部からも参照可能。

## ディレクトリ構造

```
server/
├── cmd/
│   ├── api/                    # エントリポイント(ワイヤリング + net/http)
│   └── migration-gen/      # *.gen.go(entity → Postgres up SQL の独自ジェネレータ、codegen が生成)
├── internal/
│   ├── domain/
│   │   ├── entity/             # *.gen.go(生成、GORM タグ + TableName 内蔵)
│   │   ├── repository/         # *_repository.gen.go(interface、生成)
│   │   │   └── mock/           # mock_*_repository.gen.go(テスト用スタブ、生成)
│   │   └── service/            # ドメインサービス(★手書き)
│   ├── dto/                    # *.gen.go(DTO + From<Entity> 変換、生成)
│   ├── usecase/                # *_usecase_interface.gen.go(生成) + <Name>UsecaseImpl(★手書き)
│   ├── handler/                # *_handler.gen.go(REST Handler、生成)
│   ├── infra/
│   │   ├── postgres/           # errors.gen.go(errs.DBErrorChecker の Postgres 実装、生成)
│   │   └── repository/         # *_postgres_repository.gen.go(GORM、生成)
│   └── di/
│       └── handlers.gen.go     # Handlers struct + Register(mux)(生成)
├── pkg/
│   └── util/                   # env / logger(★手書き) + errs / tx(*.gen.go、codegen が生成するランタイム)
├── migrations/                 # Postgres up SQL + .snapshot.json(migration-gen 出力 + ★git commit)
└── docs/
    ├── spec/                   # 仕様書
    ├── work/                   # 実装計画書
    └── knowledge/              # ナレッジ
```

> `proto/` はプロジェクトのルート（`../`）、生成器（codegen）は `../../` にある。FE・OpenAPI と同じ proto・同じ解釈を共有するため。生成物が proto と一致しているかは `make -C ../.. proto-check` で検査できる。

## proto アノテーションと生成ルール

アノテーション（`@entity` / `@pk` / `@unique` / `@paging` / `@required` / `@email` / `@timestamp` / `@decimal(p,s)` / `@http`）の意味、Response 形状（単一 entity → DTO、repeated → DTO 配列、それ以外 → `<Method>Output`、空 → 204）、path / query / body の振り分けは **`../../docs/spec.md` を必ず読むこと**（正本。ここには重複して書かない）。

## コーディング規約

- **命名**:
  - パッケージ: 小文字（`entity`, `service`, `usecase`, `handler`）
  - インターフェース: ドメイン名を含めない（`repository.UserRepository` は OK、`user.UserRepository` は冗長）
- **エラー**: 標準 `errors` パッケージで生成。文脈が必要なら `fmt.Errorf("context: %w", err)`
- **コンテキスト**: `context.Context` は第一引数
- **時刻取得**: コンストラクタで注入された `clock func() time.Time` を経由（テスト容易性のため）
- **コメント**: 原則書かない。書く場合は WHY のみ。公開 API は godoc

## よく使うコマンド

- **ツールインストール**: `make install-tools`(golang-migrate + migration-gen)
- **Proto 生成**: `make proto-gen`(proto 変更後は必須。codegen の `make proto-gen` を呼ぶ)
- **生成物削除**: `make proto-clean`
- **Migration 生成**: `make migration-gen`(entity 変更後、Postgres up SQL を `migrations/` に出力)
- **Migration 生成 (dry-run)**: `make migration-gen-dry`(SQL を標準出力)
- **ビルド**: `make build`
- **実行**: `make run`
- **テスト**: `make test`
- **Lint**: `make lint`
- **フォーマット**: `make fmt`
- **Docker 起動**: `make docker-up`

## Migration 生成 (`cmd/migration-gen`)

Entity の `gorm:` タグから Postgres 向け **up migration SQL** を自動生成する独自ツール。`make migration-gen` で起動。ツール自体も `make proto-gen` が生成する（`*.gen.go`、手で編集しない）。

- **入力**: `internal/domain/entity/registry.gen.go` の `entity.All`(proto 生成物。全 `@entity` 構造体の zero-value ポインタが入る)
- **状態管理**: `migrations/.snapshot.json`(前回適用した schema を JSON で保持。**git commit する**)
- **出力**: `migrations/<YYYYMMDDHHMMSS>_<table>.up.sql`(1 entity の差分につき 1 ファイル)
- **適用**: [`golang-migrate/migrate`](https://github.com/golang-migrate/migrate) を採用想定。`migrate -path migrations -database $DATABASE_URL up`
- **down は手書き**: 自動生成しない。down が必要な場合は `<同 timestamp>_<table>.down.sql` を手書きする

### 差分判定ルール

| ケース | 出力 |
|---|---|
| snapshot が空 / 該当 entity 不在 | `CREATE TABLE` |
| 既存 entity に新カラム | `ALTER TABLE ... ADD COLUMN` |
| 既存 entity からカラム消滅 | `ALTER TABLE ... DROP COLUMN` (WARNING コメント付き) |
| 型 / NOT NULL / unique 変化 | `ALTER COLUMN` / `ADD CONSTRAINT` / `DROP CONSTRAINT` |
| entity が消滅 | `DROP TABLE` (WARNING コメント付き) |

### 規約

- **CASCADE は使わない**: `DROP TABLE` / `DROP COLUMN` で依存があれば失敗する。FK 参照や view からの依存は手動で剥がしてから適用
- **DEFAULT は使わない**: gorm タグの `default:` は無視される。NOT NULL カラムを後から追加する場合は **2 段階**(NULL 許容で追加 → 値を埋める → NOT NULL 化)が必要、生成 SQL に WARNING コメントが入る
- **危険操作のガード**: NOT NULL 追加 / DROP COLUMN / DROP TABLE / 型変更には `-- WARNING:` ヘッダが付与される。確認のうえ手動で確認・必要なら修正してから commit
- **snapshot とコードの整合性**: `migrations/.snapshot.json` は entity 状態の真実。手動で migration SQL を書いた場合、snapshot を手で同期しないと次回 diff で差分が再出現する

### `AutoMigrate` との使い分け

`internal/infra/repository/*_postgres_repository.gen.go` には `AutoMigrate<Name>(db *gorm.DB)` も生成されているが、**integration test でスキーマを用意するためのヘルパー**。

**`cmd/api/main.go` は環境を問わず `AutoMigrate` を呼ばない。** GORM が期待する制約名（`uni_<table>_<column>`）と `migration-gen` が出す SQL の制約名（Postgres 既定の `<table>_<column>_key`）が食い違うため、migration 適用済みの DB に対して AutoMigrate を走らせると `DROP CONSTRAINT` に失敗して起動できない。スキーマ適用は `make migration-gen` → `make migrate-up` の一本道にする。

## 規約上の禁則

- `cmd/api/main.go` 以外で具体的な Repository 実装を import しない(依存注入は main でのみ)
- Service / UseCase 層から直接 DB に触らない(Repository 経由)
- Entity に依存性を持たせない(GORM タグ + バリデーションロジックのみ。他層を import しない)
- **`*.gen.go` ファイルを手動編集しない**(Entity / DTO / Repository / Mock / Postgres 実装 / Usecase interface / REST Handler / DI 配線 / entity registry はすべて proto から生成)
- **`migrations/*.up.sql` と `migrations/.snapshot.json` の関係を壊さない**: SQL を手書き修正したら snapshot も手動で同期する。snapshot だけ書き換えて差分を消すのは厳禁(本番 DB と乖離する)
- Handler を手書き追加しない(生成物で十分、複雑な変換が必要なら Usecase 実装に寄せる)
- **Usecase は entity を直接返さない**。クライアントへ抜ける戻り値は必ず `dto.From<Name>` で DTO に変換してから返す
- **Handler は DTO の整形をしない**。`json.Encode(usecaseResult)` だけ。json タグの責務は DTO のみ
- proto 変更後は必ず `make proto-gen` を実行
- Entity に振る舞いを足したい場合は `<snake>_ext.go` で拡張する(生成ファイルとは別ファイル)
- `internal/` 配下のパッケージを外部モジュールから import しない(Go の internal 規則で物理的に禁止される)

## コミット・PR

- **ブランチ**: `feat/<Issue番号>-<topic>` / `fix/<Issue番号>-<topic>`（例: `feat/42-add-login`）
- **メッセージ**: Conventional Commits（`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`）
- **件名言語**: 英語 or 日本語（プロジェクトで統一）
