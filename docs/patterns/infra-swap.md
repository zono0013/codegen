# インフラ層の切り替えパターン（Postgres ↔ MySQL / Redis / MongoDB）

mss-protoc-gen は既定で **PostgreSQL + GORM の単一実装** を生成する。運用中に別 DB / KVS へ移管したくなった場合、**テンプレート (`.tpl`) と CLAUDE.md を書き換えることで切り替える**。InMemory とのランタイム DI 切り替えは採用しない（ユニットテストは Mock、結合テストは testcontainers の Postgres で完結させる）。

## 全体像

```
       [唯一の永続化実装]
             │
             ▼
   PostgresUserRepository (生成、GORM)
             │
             ▼
        UserRepository (interface, 生成)
             ▲
             │
      Service / UseCase (手書き)
         （実装に依存しない）
```

DB を MySQL / Redis / MongoDB に変えるときは、Repository interface と Mock はそのまま使え、**切り替え対象は以下 3 つだけ**:

1. `mss-protoc-gen` の infra 実装テンプレート (`<kind>/infra_postgres_repository/...`)
2. CLAUDE.md の「技術スタック > データストア」の記述
3. ランタイムの接続ドライバ依存（`go.mod` / `package.json`）

## 変更手順

### 1. 生成テンプレートを差し替える

既存の Postgres 用テンプレートを別ディレクトリにフォークし、`buf.gen.yaml` から参照先を変える。

```
internal/gogen/generator/
├── infra_postgres_repository/output/pkg/infra/repository/<name>_postgres_repository.gen.go.tpl
└── infra_mysql_repository/output/pkg/infra/repository/<name>_mysql_repository.gen.go.tpl    ← 新規
```

テンプレート内で差し替えが必要になる典型箇所:

| 変更対象 | Postgres | MySQL | Redis | MongoDB |
| --- | --- | --- | --- | --- |
| ドライバ | `gorm.io/driver/postgres` | `gorm.io/driver/mysql` | `github.com/redis/go-redis/v9` | `go.mongodb.org/mongo-driver/mongo` |
| ORM スタイル | GORM | GORM | 生クライアント（KV 操作） | BSON + mongo-go-driver |
| 型マッピング | `timestamptz` | `datetime(6)` | string（JSON エンコード）| BSON Date |
| `SelectAll` | `Find` | `Find` | `SCAN` + `MGET` | `Find` + cursor |
| `Insert` の重複検知 | `23505` / `pgconn.PgError` | `1062` / `*mysql.MySQLError` | `SET NX` の戻り値 | Duplicate key error（`11000`） |
| 一意制約 | `uniqueIndex` タグ | `uniqueIndex` タグ | セカンダリインデックスを別キーで構築 | `CreateIndex` |

**KVS（Redis）への移管時の注意**: `SelectAll` / `SelectByUnique` / `BulkUpsert` などは全件スキャンや副次インデックスの自前管理が必要。そのままの interface を残すか、KVS に向いた interface へ絞り込むかをチームで判断する。interface を絞る場合は proto アノテーションから見直す（@unique を付けたフィールド単位で副次インデックスを生成するなど）。

**Document DB（MongoDB）への移管時の注意**: Bulk 操作は `InsertMany` / `BulkWrite` にマップできる。テーブル名 → コレクション名へ概念を置き換える。

### 2. CLAUDE.md を更新する

各プロジェクト直下の `CLAUDE.md` には **技術スタックの正本** が書かれている。切り替え時は以下を更新:

```diff
 ## 技術スタック
-- **データストア**: PostgreSQL (GORM `gorm.io/gorm` + `gorm.io/driver/postgres`)
+- **データストア**: MySQL (GORM `gorm.io/gorm` + `gorm.io/driver/mysql`)
```

Claude Code などのエージェントは **CLAUDE.md を起動時に読む** ため、ここを更新すれば以降の計画立案・実装・レビューは新しい DB を前提にしてくれる。

### 3. ドライバ依存を入れ替える

- **Go**: `go mod edit -droprequire=gorm.io/driver/postgres && go get gorm.io/driver/mysql` のように置換
- **TS**: `npm uninstall pg && npm install mysql2` のように置換

### 4. 接続処理（`main` / `container`）を更新

生成された `NewPostgres<Entity>Repository` の名前がテンプレート変更に伴って `NewMysql<Entity>Repository` 等に変わるため、エントリポイントの import / 呼び出しを合わせて更新する。

```go
// Before
db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})
userRepo := infrarepo.NewPostgresUserRepository(db)

// After (MySQL)
db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{})
userRepo := infrarepo.NewMysqlUserRepository(db)
```

### 5. `make proto-gen` を実行して再生成

テンプレート変更後は必ず再生成。コンパイルエラーが出た箇所が手書き側の追従対象。

## 何が変わらないか

- **Repository interface（`*_repository.gen.*`）**: DB が何であろうと Service / UseCase は同じシグネチャを呼ぶ
- **Mock（`mock_*_repository.gen.*`）**: ユニットテストには影響しない
- **Entity（`*.gen.*`）**: ドメイン層は DB 非依存
- **Service / UseCase / Handler**: interface に依存しているため、全く触らない

## 注意点

- 生成物 (`*.gen.*`) は手で編集しない（次回の `make proto-gen` で上書きされる）
- 本番マイグレーションは `AutoMigrate` ではなく Goose / Atlas / Flyway 等のマイグレーションツールで管理する
- Redis / MongoDB のように DB パラダイムが変わる場合、既定の CRUD interface を全て満たせないことがある。その場合は proto アノテーション・interface 生成規約の見直しから入る
- 複雑なクエリ（JOIN / 集計 / pipeline）が必要な場合は `*_postgres_repository_ext.go` のような拡張ファイルに手書きメソッドを切り出す（生成物の interface 実装は触らない）

## フレームワーク依存外の一般パターンとして

この構成は Go / Hono / Next どれでも成立する。共通コンセプト:

1. **ドメインは interface にしか依存しない**
2. **interface は可能なら自動生成で固定する**
3. **永続化実装は生成物 1 つに絞る**（テストは Mock / testcontainers で賄う）
4. **DB 変更は「テンプレート + CLAUDE.md の書き換え」= `proto-gen` 再実行**

proto を使わないプロジェクトでも、手書きの interface に対して Postgres 実装を 1 つ用意し、CLAUDE.md で宣言する構成にすれば同じ恩恵が得られる。
