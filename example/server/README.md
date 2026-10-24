# server

Go + DDD + クリーンアーキテクチャ + REST のバックエンド。[codegen](../../README.md) で proto から DDD の各層を生成するプロジェクトのバックエンド。

> 設計の考え方は [`../../docs/architecture.md`](../../docs/architecture.md)、生成の仕様は [`../../docs/spec.md`](../../docs/spec.md) を参照。

## セットアップ

```bash
make install-tools          # golang-migrate + migration-gen をインストール
make proto-gen              # proto からコード生成 (entity / repository / handler / usecase / dto / DI)
go mod tidy                 # 依存解決
cp .env.example .env.local  # 起動前に環境変数ファイルを用意 (APP_ENV=local で読まれる)
docker compose up -d db     # Postgres を起動
make migration-gen          # entity の gorm タグから up SQL を生成 (初回 = CREATE TABLE)
make migrate-up             # DB にスキーマを適用
make run                    # ローカル起動 (:8080)
```

> スキーマ適用は migration の一本道。`cmd/api/main.go` は起動時に `AutoMigrate` を呼ばない
> （GORM が期待する制約名と生成 SQL の制約名が食い違い、migration 済みスキーマを壊すため）。
> 生成される `AutoMigrate<Entity>` は integration test 用のヘルパー。

## 共通ユーティリティ（`pkg/util/`）

### `env` — 環境変数ローダー

```go
import "example.com/blog/server/pkg/util/env"

env.AppEnv()   // "local" | "dev" | "staging" | "prod"
env.DBHost(), env.DBPort(), env.DBUser(), env.DBPassword(), env.DBName()
env.Port()     // ":8080" (default)
```

起動時に `APP_ENV` を見て `.env.<APP_ENV>` を読み込む（未設定なら `local`）。必須キー欠落時は getter 呼び出し時に panic。

### `logger` — 構造化ロガー（`log/slog`）

```go
import "example.com/blog/server/pkg/util/logger"

logger.Init()                              // main の最初に 1 度
logger.Info(ctx, "msg", "user_id", uid)    // ctx に積んだ attrs も展開
ctx = logger.WithAttrs(ctx, slog.String("request_id", rid))
```

`local`/`dev` → text / stderr / debug、`staging`/`prod` → JSON / stdout / info。

### `tx` — トランザクション境界

```go
import "example.com/blog/server/pkg/util/tx"

tx.Init(db)                                             // main で 1 度
err := tx.Run(ctx, func(ctx context.Context) error {    // Usecase 層で境界を張る
    if err := repoA.Insert(ctx, a); err != nil { return err }
    return repoB.Insert(ctx, b)                         // 同一 tx で実行される
})
```

`tx` と `errs` は codegen が生成するランタイム（`*.gen.go`、手で編集しない）。エラーの種類を増やすときは、パッケージ変数の初期化で `var ErrorTypeTeapot = errs.Register("TEAPOT", http.StatusTeapot, "I'm a teapot")` のように登録する（`init` ではなく）。

生成された Repository は内部で `tx.From(ctx)` を呼ぶので、`tx.Run` の中では自動的に同一トランザクションを使う。`tx.Run` の外で呼ばれた場合は default pool + `slog.Warn` 出力。

## エンドポイント (REST)

URL は proto の rpc 毎に `@http METHOD /path` アノテーションで宣言済み。生成物の `internal/di/handlers.gen.go` が `mux.HandleFunc` で登録する。レスポンスは `internal/dto/*.gen.go` の DTO を `json.Encode` した snake_case JSON。

| Method | Path | RPC |
|---|---|---|
| GET | `/api/users` | `ListUsers` |
| GET | `/api/users/cursor` | `ListUsersByCursor` (`?limit=&after_id=`) |
| GET | `/api/users/by-email` | `GetUserByEmail` (`?email=`) |
| GET | `/api/users/{id}` | `GetUser` |
| POST | `/api/users` | `CreateUser` |
| POST | `/api/users/bulk` | `BulkCreateUsers` |
| PUT | `/api/users/bulk` | `BulkUpsertUsers` |
| PUT | `/api/users/{id}` | `UpsertUser` |
| PATCH | `/api/users/{id}` | `UpdateUser` |
| POST | `/api/users/bulk-delete` | `BulkDeleteUsers` |
| DELETE | `/api/users/{id}` | `DeleteUser` |
| DELETE | `/api/users` | `DeleteAllUsers` |
| GET | `/health` | (手書き) |

```bash
# ユーザー作成
curl -X POST http://localhost:8080/api/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","name":"Alice"}'

# ユーザー取得
curl http://localhost:8080/api/users/<uuid>

# cursor pagination
curl 'http://localhost:8080/api/users/cursor?limit=20&after_id=<last-id>'

# ヘルスチェック
curl http://localhost:8080/health
```

## Docker

```bash
make docker-up       # api + postgres を起動
make docker-down     # 停止
```

## ユースケース別ガイド

proto 駆動の開発でよくある作業の手順。

### 新しいエンティティを追加したい

User のような新ドメイン概念(例: `Order`)を追加する。

1. `proto/order/v1/order.proto` を作成し、`// @entity` マーカー付きで定義。rpc には `// @http METHOD /path` を付ける
   ```proto
   // @entity
   message Order {
     // @pk @paging
     string id = 1;
     // @unique
     string order_number = 2;
     // @timestamp
     int64 created_at_unix = 3;
   }

   service OrderService {
     // @http POST /api/orders
     rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
     // @http GET /api/orders/{id}
     rpc GetOrder(GetOrderRequest) returns (GetOrderResponse);
   }
   ```
2. `make proto-gen` → 以下が生成される:
   - `internal/domain/entity/order.gen.go` (構造体 + GORM タグ)
   - `internal/domain/entity/registry.gen.go` (`entity.All` に `&Order{}` が追加される)
   - `internal/domain/repository/order_repository.gen.go`
   - `internal/domain/repository/mock/mock_order_repository.gen.go`
   - `internal/infra/repository/order_postgres_repository.gen.go`
   - `internal/dto/order.gen.go` (`OrderDTO` + `FromOrder` / `FromOrders`)
   - `internal/usecase/order_usecase_interface.gen.go`
   - `internal/handler/order_handler.gen.go`
   - `internal/di/handlers.gen.go`
3. `make migration-gen` → `migrations/<YYYYMMDDHHMMSS>_create_orders.up.sql` が出る + `migrations/.snapshot.json` に orders が追加される(両方 git commit する)
4. `internal/domain/service/order_service.go` を手書き(ビジネスルール)
5. `internal/usecase/order_usecase.go` に `OrderUsecaseImpl` を手書き(生成 interface を実装。戻り値は `*dto.OrderDTO` / `[]*dto.OrderDTO` で、`dto.FromOrder*` で entity から変換)
6. `cmd/api/main.go` の `di.NewHandlers(...)` 呼び出しに `orderUsecase` を追加

#### proto アノテーション

`@entity` / `@pk` / `@unique` / `@paging` / `@timestamp` / `@decimal(p,s)` / `@http METHOD /path` などの仕様は [`../../docs/spec.md`](../../docs/spec.md#proto-アノテーション) を参照。

### 既存エンティティにフィールドを追加したい

User に `phone_number` を追加するケース。

1. `proto/user/v1/user.proto` にフィールド追加(必要なら `// @required` 等のマーカー)
2. `make proto-gen` で `user.gen.go` が再生成 → ファクトリ `NewUser` のシグネチャと DTO が変わる
3. **コンパイルエラー** で影響範囲(Service / Handler / Usecase 実装)が特定される
4. エラーが出た箇所を最小差分で更新
5. `make migration-gen` で `migrations/<timestamp>_alter_users.up.sql` が出る — `ADD COLUMN` の場合 NOT NULL なら **WARNING コメント** が付くので、既存行があるなら 2 段階(NULL 許容で追加 → 値埋め → NOT NULL 化)に手動分割する判断をここで行う
6. SQL を確認したら `migrations/.snapshot.json` ごと commit
7. 本番適用は `migrate -path migrations -database "$DATABASE_URL" up`

### 新しい REST エンドポイントを追加したい

`UserService` に `UpdateUser` を追加するケース。

1. `proto/user/v1/user.proto` の `service` ブロックに rpc を追加、`// @http PUT /api/users/{id}` を付ける
2. `make proto-gen` → `UserUsecase` interface にメソッドが足され、生成 Handler と DI の `Register` に URL が追加される
3. `internal/usecase/user_usecase.go` の `UserUsecaseImpl` に新メソッドを実装(interface 要件を満たさないとコンパイルエラー)
4. 必要に応じて UseCase / Service にメソッド追加

### PostgreSQL から別 DB（MySQL / Redis / MongoDB）へ移管したい

永続化は PostgreSQL + GORM の単一実装。別の DB に移すには codegen のテンプレートとランタイムを差し替える（codegen に PR を出すかフォークする）。差し替える場所は [`architecture.md` の「DB を変えたいとき」](../../docs/architecture.md#db-を変えたいとき) を参照。移した後は、この README と `CLAUDE.md` の「データストア」欄も書き換える。

## レイヤー構成 (`*.gen.*` が自動生成、それ以外が手書き)

```
proto/user/v1/user.proto  ← 唯一の手書き source
     │
     ├─ make proto-gen (../../ の codegen)
     │   │
     │   ├→ internal/domain/entity/user.gen.go              (struct + GORM タグ + TableName)
     │   ├→ internal/domain/entity/registry.gen.go          (entity.All 一覧)
     │   ├→ internal/domain/repository/user_repository.gen.go
     │   ├→ internal/domain/repository/mock/mock_user_repository.gen.go
     │   ├→ internal/infra/repository/user_postgres_repository.gen.go
     │   ├→ internal/dto/user.gen.go                         (UserDTO + From*)
     │   ├→ internal/usecase/user_usecase_interface.gen.go
     │   ├→ internal/handler/user_handler.gen.go             (REST Handler、net/http)
     │   └→ internal/di/handlers.gen.go                      (Register(mux))
     │
     └─ make migration-gen (cmd/migration-gen)
         │
         └→ migrations/<timestamp>_<create|alter|drop>_<table>.up.sql  (差分ベース、CASCADE/DEFAULT は使わない)
            migrations/.snapshot.json                        (前回適用 schema、git commit)

REST Handler (生成) ─json.Encode→ DTO (生成)
        ↓ 呼び出し                  ↑ 変換 (usecase 内で dto.From*)
Usecase interface (生成)           Entity (生成)
        ↑ 実装                       ↑ GORM が直接読み書き
<Name>UsecaseImpl (手書き) → Service (手書き) → Repository (生成)
                                                     ↑ 実装
                                          Postgres Repository (生成)
```

HTTP リクエストは生成 Handler が JSON / query を parse → Usecase 実装に委譲 → Entity を Repository 経由で操作 → Usecase 内で DTO に変換 → Handler が `json.Encode(dto)` で返却、の順で通ります。

## インフラ層の構成 (Postgres 単一実装)

Repository の本番実装は proto から生成された **PostgreSQL + GORM 版**(`internal/infra/repository/*_postgres_repository.gen.go`)に統一しています。InMemory 実装は採用しません。

- **ユニットテスト**: 生成された Mock(`internal/domain/repository/mock/`)を Service / UseCase のテストに注入
- **結合テスト**: docker-compose または testcontainers で起動した PostgreSQL に接続し `tx.Init(db)` を呼んでから `NewPostgresUserRepository(rdbWrapper)` で取得。テスト用 schema は `AutoMigrateUser(db)` で同期(本番は使わない)
- **本番**: 同じ `NewPostgresUserRepository(rdbWrapper)`。DB 接続情報は `env.DB*()` から。スキーマは `migrations/*.up.sql` を `golang-migrate` で適用

### 配線例 (`cmd/api/main.go`)

```go
import (
    "gorm.io/driver/postgres"
    "gorm.io/gorm"

    infrapostgres "example.com/blog/server/internal/infra/postgres"
    infrarepo "example.com/blog/server/internal/infra/repository"
    "example.com/blog/server/pkg/util/errs"
    "example.com/blog/server/pkg/util/env"
    "example.com/blog/server/pkg/util/logger"
    "example.com/blog/server/pkg/util/tx"
)

logger.Init()
dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
    env.DBHost(), env.DBPort(), env.DBUser(), env.DBPassword(), env.DBName())
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
if err != nil { log.Fatal(err) }
tx.Init(db)
// 本番では AutoMigrateUser を呼ばない。代わりに migrations/*.up.sql を golang-migrate で適用しておく。
rdbWrapper := errs.NewRDBErrorWrapper(infrapostgres.NewPostgresErrorChecker())
userRepo := infrarepo.NewPostgresUserRepository(rdbWrapper)
```

## Migration 運用 (`migrations/`)

`make migration-gen` で Entity の `gorm:` タグから差分の SQL を作り、`make migrate-up` で適用する。規則は [`../../docs/spec.md` の「マイグレーション」](../../docs/spec.md#マイグレーションmigration-gen) を参照。
