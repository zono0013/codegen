# 設計

codegen が生成するコードの構造と、その理由。何が生成されるかの正確な仕様は [`spec.md`](spec.md)。

## 考え方

- **proto が唯一のソース**: ドメインのデータ（`@entity` の message）と API（`@http` の rpc）を `proto/` に書く。そこから server（Go）・front（TypeScript）・OpenAPI を同じ解釈で生成するので、3 つが食い違わない
- **型で影響範囲が出る**: proto を変えると生成物が変わり、それを使う手書きのコードがコンパイルエラーになる。フィールドの追加や改名で、直す場所の漏れが起きない
- **手書きは振る舞いだけ**: 定型の層は生成し、手で書くのはドメインのルール（Service）とユースケース（Usecase 実装）、画面だけにする

## server の層

```
proto ──make proto-gen──┐
                        ▼
REST Handler（生成）── Usecase interface（生成）
                              ▲ 実装
                       Usecase 実装（手書き）── DTO（生成。Entity → JSON の形）
                              │
                       Service（手書き。ドメインのルール）
                              │
                       Repository interface（生成）
                              ▲ 実装
                       Postgres Repository（生成。GORM）── Entity（生成。GORM のモデルを兼ねる）
```

- **依存は内側へ**: Handler → Usecase → Service → Repository interface → Entity。Entity は他の層を import しない
- **抽象に依存する**: Service は Repository の interface だけを知る。Postgres の実装を組み立てるのは `cmd/api/main.go` だけ
- **JSON の形は DTO が持つ**: Usecase が Entity を DTO に変えて返し、Handler はそれを JSON にするだけ。Entity は JSON のタグを持たない
- **エラー**: Service はルール違反を `errs` の `DomainError`（`errs.NewValidationError` / `errs.NewNotFoundError` など）で返す。Repository は DB のエラーを `errs.RDBErrorWrapper` で `DomainError` に変える。Handler は `DomainError` を RFC 9457 の Problem Details（ステータスと title）にして返す
- **トランザクション**: 境界は Service が張る（`tx.Run`。テストでは素通しの関数を渡す）。Repository は `tx.From(ctx)` で同じトランザクションを使う
- **テスト**: Service / Usecase のユニットテストは生成した Mock Repository を使う。実 DB が要るテストは testcontainers などで Postgres を立てる
- **ランタイム**: 生成コードが依存する `errs`（RFC 9457 の Problem Details）、`tx`（トランザクション境界）、Postgres のエラー判定、マイグレーション生成ツールも生成する（[`spec.md` のランタイム](spec.md#ランタイム)）

## front

- `generate/` に Zod スキーマ・型と、型付きの API 関数を生成する。API のレスポンスの形は server の Handler と同じ規則（[`spec.md` の REST Handler の振る舞い](spec.md#rest-handler-の振る舞いinternalrest)）
- HTTP クライアント（`apiClient`）は生成しない。認証やリトライはプロジェクトの方針なので、プロジェクトが持つ

## スキーマの変更

DB のスキーマは、Entity の `gorm` タグから差分の SQL を作って適用する一本道にしている（`go run ./cmd/migration-gen` で SQL を作り、golang-migrate などで適用する）。起動時の `AutoMigrate` は使わない。規則は [`spec.md` のマイグレーション](spec.md#マイグレーションmigration-gen)。

## 向き・不向き

**向いている**

- CRUD が中心の、リソース指向の API
- 同じ形の Entity がたくさんある
- 骨格が安定していて、追加・拡張が多い

**向いていない**

- Entity が少なく、それぞれの構造が大きく違う
- ルールの多くが Entity 自体の振る舞いで、生成コードに収まらない

**代償**

- 生成器（このリポジトリ）を保守する必要がある
- 生成の規則に収まらないものは手で書く。その量が多いなら、この構成が合っていない
- proto に独自のアノテーション（`@entity` など）を書くので、読む人に規約の知識が要る

## DB を変えたいとき

永続化は Postgres（GORM）だけを生成する。別の DB にするには、codegen の次の部分を差し替える。

- **GORM で扱える DB（MySQL など）**
  - Repository 実装のテンプレート: `internal/gogen/generator/infra_postgres_repository/`
  - DB のエラー判定（ランタイム）: `internal/gogen/runtime/internal/infra/postgres/`
  - マイグレーションの SQL 出力: `internal/gogen/runtime/cmd/migration-gen/sql.go.src`
- **GORM で扱えない DB（Redis、MongoDB など）**: 上に加えて、GORM に依存している部分も差し替える
  - Entity のテンプレート（`gorm` タグ）: `internal/gogen/generator/entity/`
  - トランザクション境界（ランタイム、`*gorm.DB` に依存）: `internal/gogen/runtime/pkg/util/tx/`
  - DB のエラーの変換（ランタイム）: `internal/gogen/runtime/pkg/util/errs/rdb_wrapper.go.src`、`checker.go.src`
  - マイグレーション生成（`gorm.io/gorm/schema` で Entity を読む）: `internal/gogen/runtime/cmd/migration-gen/`

生成物は `make proto-gen` のたびに上書きされるので、使う側で生成物を直しても元に戻る。DB を変えるには、codegen に PR を出すかフォークする。
