# mss-protoc-gen: DDD 層を生成するカスタムプラグイン

`proto-driven-ddd.md` で説明した「proto からドメイン層を自動生成する」方針を実現するカスタム protoc プラグインの設計・実装ガイド。

## プラグインが生成するもの

### エンティティ由来（`@entity` マーカー付きメッセージ 1 つにつき）

| ファイル | 役割 |
| --- | --- |
| `entity/<name>.gen.*` | 構造体 / クラス + ファクトリ関数（バリデーション付き） + 永続化層からの `Hydrate<Name>` ファクトリ |
| `repository/<name>_repository.gen.*` | Repository interface + センチネルエラー（`Err<Name>NotFound` / `Err<Name>AlreadyExists`） |
| `repository/mock/mock_<name>_repository.gen.*` | テスト用スタブ（Func 差し替え or `vi.fn()`） |
| `infra/<name>_postgres_repository.gen.*` | PostgreSQL 実装（ORM ベース、既定の本番実装） |

### Service 由来（service 1 つにつき）

| ファイル | 役割 |
| --- | --- |
| `usecase/<service>_usecase_interface.gen.*` | Usecase interface + 各 rpc の Input 型（Request を展開した struct / type） |
| `handler/<service>_handler.gen.*` (Go/Hono) | Connect Handler 実装（Usecase を呼び、proto ↔ Entity の変換・エラー → Connect Code マップを含む） |
| `app/<path>/route.ts` (Next のみ) | REST Route Handler（`@http METHOD /path` から path と HTTP メソッドを解決） |

### プロジェクト全体（1 ファイルだけ）

| ファイル | 役割 |
| --- | --- |
| `di/handlers.gen.go` (Go) | `Handlers` struct + `NewHandlers(deps) *Handlers` + `(h *Handlers) Register(mux)` |
| `lib/handler-registry.gen.ts` (Hono) | `HandlerDeps` 型 + `registerHandlers(router, deps)` |
| `lib/handler-registry.gen.ts` (Next) | `HandlerDeps` 型 + `provideHandlerDepsFactory()` + `getHandlerDeps()`（遅延初期化） |

生成される永続化実装は **PostgreSQL の単一実装のみ**。ユニットテストは mock 経由で行い、実 DB に触る結合テストは testcontainers 等で別途構築する（この生成物はカバーしない）。

**手書きに残るのは**:
- **Service 層**（ドメインロジック、`pkg/domain/service/*.go` / `src/domain/service/*.ts`）
- **Usecase 実装クラス**（`<Name>UsecaseImpl`、生成された interface を実装して Service を呼ぶ）
- **Entity の拡張メソッド**（必要時のみ `*_ext.go` / `*-ext.ts`）
- **DI ワイヤリング**（`cmd/api/main.go` / `src/main.ts` / `src/lib/container.ts` — リポジトリ → Usecase → 生成 DI ヘルパーの接続数行のみ）

**別 DB / KVS への移管は「テンプレート / CLAUDE.md の書き換え」として扱う** — 詳細は [infra-swap.md](./infra-swap.md) を参照。

各言語・フレームワーク向けに同じ仕様でプラグインを実装できる。Go なら protoc 公式 Go ライブラリ、TypeScript なら `@bufbuild/protoplugin`、Python なら protobuf 公式 Python ライブラリ、などを使う。

## 実装の 4 ステップ

### Step 1: proto アノテーション仕様の確定

既に `proto-driven-ddd.md` に定義がある:

```
@entity / @pk / @unique / @email / @required / @timestamp   (メッセージ / フィールド向け)
@http METHOD /path                                           (rpc 向け、Next の REST 生成で使用)
```

すべてコメント解釈方式。proto options を使わないのは、options 定義の proto 自体を先にコンパイルするブートストラップが不要になるため。`@http` は 3 実装(Go / Hono / Next)すべてで解釈され、各言語の REST フレームワーク向けに URL が登録される。

### Step 2: proto コメントの読み取り

プラグインが proto descriptor から各メッセージ / フィールドのコメントを取得する。

- **Go (`protogen`)**: `msg.Comments.Leading`, `field.Comments.Leading`
- **TS (`@bufbuild/protoplugin`)**: `message.getComments().leading`, `field.getComments().leading`

コメントに `@entity` が含まれる message のみ生成対象とする。

### Step 3: 型マッピング

| proto 型 | Go | TypeScript |
| --- | --- | --- |
| string | string | string |
| int32 / int64 | int32 / int64 | number |
| bool | bool | boolean |
| int64 + `@timestamp` | time.Time | Date |

`@timestamp` フィールドはフィールド名末尾の `Unix` を除去して命名（`createdAtUnix` → `CreatedAt` / `createdAt`）。

### Step 4: コード出力

テンプレートまたは `print` 系 API で 4 ファイルを書き出す。

**Entity 生成時の規約**:
- プライベートコンストラクタ + static ファクトリ（validation 込み）
- `@pk` フィールド: 空文字チェック
- `@required` かつ string: `trim` + 空文字チェック
- `@email`: `trim` + `@` を含むかチェック
- それ以外のフィールドは検証なしで通す
- **Hydrate ファクトリ**: Postgres 実装が DB から復元する際に validation を再走させないため、`Hydrate<Entity>(すべてのフィールド)` も並べて生成する。private フィールドへの直代入のみで、godoc に「DB 復元専用、通常は `New<Entity>` を使え」と明記する

**Repository interface 生成時の規約**:

以下を全件生成する（`@pk` は必須、`@unique` はあれば追加）:

| 関数 | 役割 | 由来 |
| --- | --- | --- |
| `SelectAll(ctx)` | 全件取得 | `@entity` |
| `SelectByPK(ctx, pk)` | 主キー検索（読み取りで未存在は nil を返す） | `@pk` |
| `SelectBy<Field>(ctx, v)` | `@unique` フィールドごとに 1 本 | `@unique` |
| `Insert(ctx, entity)` | 単件挿入。既存なら `ErrXxxAlreadyExists` | `@entity` |
| `BulkInsert(ctx, entities)` | 一括挿入（全件 atomic にチェック） | `@entity` |
| `Upsert(ctx, entity)` | 作成 or 上書き | `@entity` |
| `BulkUpsert(ctx, entities)` | 一括 upsert | `@entity` |
| `Update(ctx, entity)` | 単件更新。未存在なら `ErrXxxNotFound` | `@entity` |
| `Delete(ctx, pk)` | 単件削除。未存在なら `ErrXxxNotFound` | `@pk` |
| `BulkDelete(ctx, pks)` | 一括削除 | `@pk` |
| `DeleteAll(ctx)` | 全削除 | `@entity` |

あわせてエンティティごとにセンチネルエラー（Go: `ErrXxxNotFound` / `ErrXxxAlreadyExists`、TS: `XxxNotFoundError` / `XxxAlreadyExistsError`）を生成する。

**Mock 生成時の規約**:
- Repository interface と同じシグネチャを持つスタブ型を生成
- Go: 各メソッドに対応する `<Method>Func` フィールド。未設定ならゼロ値返却
- TS: 各メソッドを `vi.fn()` で初期化。呼び出し回数 / 引数を assertion で検証可能
- `var _ repository.XxxRepository = (*MockXxxRepository)(nil)`（Go）/ `implements XxxRepository`（TS）で interface 一致を保証
- **ユニットテストでは常に mock を使う**（Postgres 実装は結合テストでのみ起動）

**Postgres (ORM) 実装の規約**:

Repository interface と同じシグネチャを満たす本番用の実装を自動生成する。ORM は言語別に固定する:

| 言語 | ORM | 依存追加 |
| --- | --- | --- |
| Go | GORM (`gorm.io/gorm` + `gorm.io/driver/postgres`) | `go get gorm.io/gorm gorm.io/driver/postgres` |
| TypeScript | Drizzle ORM または TypeORM（後続で追加） | — |

Go / GORM 版の生成規約:

- **ファイル**: `pkg/infra/repository/<name>_postgres_repository.gen.go`
- **コンストラクタ**: `NewPostgres<Entity>Repository(db *gorm.DB) repository.<Entity>Repository`
- **DB モデル**: 同ファイル内に private な `<name>Model` 構造体を生成し、GORM タグを付与。Entity とは分離する（Entity は private フィールドのため直接 GORM マップできない）
- **マッピング関数**: `toModel(e *entity.<Entity>) *<name>Model` と `(m *<name>Model) toEntity() *entity.<Entity>` を生成。後者は Entity の `Hydrate<Entity>(...)` ファクトリを呼ぶ
- **テーブル名**: `(<name>Model) TableName() string { return "<snake_case_plural>" }` を生成。GORM 既定の複数化に頼らず明示する
- **タグマッピング**:
  - `@pk` → `gorm:"primaryKey"`
  - `@unique` → `gorm:"uniqueIndex"`
  - `@required` → `gorm:"not null"`
  - `@timestamp` → `time.Time` 列（`timestamptz`）
  - それ以外の列は tag 省略（デフォルト推論に任せる）
- **操作マッピング**:

  | interface メソッド | GORM 実装 |
  | --- | --- |
  | `SelectAll` | `db.WithContext(ctx).Find(&models)` |
  | `SelectByPK` | `db.WithContext(ctx).First(&model, "id = ?", pk)` — `ErrRecordNotFound` は `nil, nil` で返す |
  | `SelectBy<Unique>` | `db.WithContext(ctx).Where("<col> = ?", v).First(&model)` — 同上 |
  | `Insert` | `db.WithContext(ctx).Create(&model)`。一意制約違反（`*pgconn.PgError` の `23505`）を `ErrXxxAlreadyExists` に変換 |
  | `BulkInsert` | `db.WithContext(ctx).Transaction(...)` + `CreateInBatches` |
  | `Upsert` | `db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&model)` |
  | `BulkUpsert` | Transaction 内で上記を適用 |
  | `Update` | `db.WithContext(ctx).Save(&model)` 前に `First` で存在確認し、未存在なら `ErrXxxNotFound` |
  | `Delete` | `db.WithContext(ctx).Delete(&<name>Model{}, "id = ?", pk).RowsAffected == 0` なら `ErrXxxNotFound` |
  | `BulkDelete` | Transaction 内で `Where("id IN ?", pks).Delete(...)`。行数不足時は `ErrXxxNotFound` でロールバック |
  | `DeleteAll` | `db.WithContext(ctx).Where("1 = 1").Delete(&<name>Model{})` |

- **Bulk 操作の atomic 性**: すべて `db.Transaction` でラップし、どれか 1 件が失敗したら全件ロールバックされるようにする
- **マイグレーション**: `db.AutoMigrate(&<name>Model{})` を呼び出すヘルパー（例: `AutoMigrate<Entity>(db)`）を同ファイルに生成。本番では CLI や Goose / Atlas 等で運用してもよいが、最小構成の開発向けに同梱する
- **コンパイル時保証**: `var _ repository.<Entity>Repository = (*postgres<Entity>Repository)(nil)` を生成ファイル末尾に置く

**Usecase 生成の規約**:

service の rpc 1 本ごとに以下を生成する。Usecase は **proto に依存しない** 純粋ドメインの interface として設計する:

- **Input 型**: Request メッセージのフィールドをそのまま展開した struct / type。例: `CreateUserRequest { email, name }` → `type CreateUserInput = { email, name }`
- **戻り値**:
  - Response が単一フィールドの `<Entity>` なら `*entity.<Entity>` / `Promise<Entity | null>`（未存在を nil / null で許容）
  - Response が単一フィールドの `repeated <Entity>` なら `[]*entity.<Entity>` / `Promise<Entity[]>`
  - それ以外（multi-field / Empty）なら戻り値なし（`error` のみ）
- **手書きの `<Name>UsecaseImpl`** がこの interface を実装し、Service を呼ぶ

**Handler 生成の規約(REST、3 実装共通の骨格)**:

- rpc の leading comment の `@http METHOD /path` から URL とメソッドを決定(path パラメータは `{name}` 形式)
- body は **body JSON parse → Input 展開 → Usecase 呼び出し → Entity → JSON 整形 → レスポンス書き込み** の定型パターン
- **エラー変換**: Repository の `Err<Name>NotFound` / `Err<Name>AlreadyExists` を命名規約で `404` / `409` にマップ、その他は `400` に fallback

実装別の差:

| 実装 | フレームワーク | path 記法 | path param 取得 | body 取得 | レスポンス書き込み |
| --- | --- | --- | --- | --- | --- |
| Go | `net/http` (Go 1.22+) | `"GET /api/users/{id}"` | `r.PathValue("id")` | `json.NewDecoder(r.Body).Decode(...)` | `json.NewEncoder(w).Encode(...)` |
| Hono | Hono v4 | `/api/users/:id`(生成器が `{}` → `:` へ変換) | `c.req.param("id")` | `await c.req.json()` | `c.json(...)` |
| Next | Next.js App Router | `src/app/api/users/[id]/route.ts`(`{}` → `[]`) | `params.id` | `await req.json()` | `NextResponse.json(...)` |

Next のみ Route Handler ファイル自身が「ファイルパス = URL」なので `src/app/<path>/route.ts` に出力される。他 2 実装では `pkg/handler/*_handler.gen.go` / `src/handler/*-handler.gen.ts` に handler クラスを出し、DI 配線側で URL を登録する。

**DI 生成の規約**:

- **Go (`pkg/di/handlers.gen.go`)**: 全 Handler を束ねる `Handlers` struct、`NewHandlers(deps...)`、`(h *Handlers) Register(mux *http.ServeMux)` を生成。`main.go` は `di.NewHandlers(userUsecase).Register(mux)` の 2 行で済む
- **Hono (`src/lib/handler-registry.gen.ts`)**: `HandlerDeps` 型 と `registerHandlers(router, deps)` 関数を生成。`main.ts` は `routes: (router) => registerHandlers(router, { userUsecase })` で登録
- **Next (`src/lib/handler-registry.gen.ts`)**: `provideHandlerDepsFactory(factory)` と `getHandlerDeps()` の遅延初期化ペアを生成。`src/lib/container.ts`（手書き）で factory を登録、Route Handler が side-effect import で読み込む

外部 DI フレームワーク（Fx / tsyringe 等）は **採用しない**。handler-registry / Handlers struct は軽量な手書きコードと同等の形で、追加依存なしに動く。

## 命名正規化

Go の lint（`stylecheck` ST1003）は initialism を期待する:
- `Id` → `ID`
- `Url` → `URL`
- `Api` → `API`
- `Http` → `HTTP`
- `Json` → `JSON`

プラグイン内で末尾マッチを検出して置換する。TypeScript では camelCase を維持（`id`, `url`, ...）。

## buf への組み込み

> このリポジトリでは buf を使わず、`cmd/gen` が proto のコンパイルから書き出しまでをプロセス内で行う（生成器が 1 言語に揃っているなら、プラグインとして分ける必要がないため）。以下は生成器が複数言語に分かれる場合の組み込み例。

```yaml
# buf.gen.yaml
version: v2
plugins:
  - local: mss-protoc-gen        # Go: GOBIN 内のバイナリ
    out: .
  - local: ["node", "./tools/mss-protoc-gen/index.mjs"]  # TS: Node スクリプト
    out: .
```

`out: .` でプロジェクトルート起点のパスで書き出す。プラグイン側で `pkg/domain/entity/<name>.gen.go` のような絶対パスを指定する。

## テンプレート構成（`.tpl` ベース）

出力コードはインラインの `Print` ではなく、`.tpl` テンプレートファイルに外出しする。これにより:
- エディタでシンタックスハイライト & 通常のファイル編集フローで修正可能
- 出力先のパスを `output/` 配下のディレクトリ構造でそのまま表現できる（目的のファイルパスをミラー）
- テンプレートファイル単位でレビュー / diff しやすい
- **別 DB / KVS に移管する際に、テンプレートファイル単位で差し替え可能**（後述）

ディレクトリ規約:

```
cmd/mss-protoc-gen/                         # Go
├── main.go                                  # proto 解析 + テンプレート呼び出し
└── generator/
    ├── <kind>/output/<mirrored-path>/<name>.gen.go.tpl
    └── ...

tools/mss-protoc-gen/                        # TypeScript
├── index.mjs                                # エントリポイント
├── template.mjs                             # 最小テンプレートエンジン
└── generator/
    └── <kind>/output/<mirrored-path>/<name>.gen.ts.tpl
```

`<kind>` は以下の 7 種:

- **エンティティ由来**: `entity` / `repository` / `mock` / `infra_postgres_repository`
- **Service 由来**: `usecase` / `handler_connect`（Go/Hono）or `handler_rest`（Next）
- **プロジェクト全体**: `di`

`output/` 以下はプロジェクトルート相対の実際の配置と一致させる。

テンプレート言語は Go の `text/template` に揃える:
- `{{.Name}}` / `{{.Field.Sub}}` — データ参照
- `{{range .Fields}}…{{end}}` — 繰り返し
- `{{if cond}}…{{else}}…{{end}}` — 条件分岐
- `{{$.Name}}` — ループ内で最上位データを参照
- `{{funcName arg1 arg2}}` / `{{and a b}}` — 関数・ビルトイン呼び出し（括弧でネスト可）

Go は `//go:embed generator` + `text/template`、TS は `template.mjs` の最小実装（上記サブセットを解釈）で同じ記法を扱う。

## 既存実装の参照ポイント

このリポジトリ（codegen）が参照実装で、仕様の正本は [`docs/spec.md`](../spec.md)。Go 向け・TS 向け・OpenAPI 向けのジェネレータを 1 つの Go モジュールに置き、アノテーションの解釈（`internal/annotation`）と Handler の振る舞い（`internal/rest`）を共有することで、BE / FE / API 仕様の食い違いを構造的に防いでいる。別言語（Python / Rust など）で新たに実装する場合もこの `.tpl` 方針に揃える。

## 拡張の方向性

必要になったら追加できるマーカー例:
- `@foreign_key=<target>` — 別エンティティへの参照を表現
- `@soft_delete` — 論理削除列（`deleted_at`）を自動付与
- `@audit` — 作成者 / 更新者列を自動付与
- `@index` — DB スキーマ生成時のインデックス指定

段階的に導入し、「書いたコードのうち何割が手書き → 自動生成に移行できたか」を指標にする。
