# mss-migration-gen: Entity から DB マイグレーションを生成するカスタムジェネレータ

`mss-protoc-gen` が `.proto` から DDD 層を生成するのに対し、`mss-migration-gen` は **生成された Entity を入力に DB スキーマの差分マイグレーション SQL を生成する** カスタムツール。proto-driven-ddd の続きで、`Entity` と DB スキーマを単一の真実(`.proto` + ORM タグ)から導く役割を担う。

## このパターンが扱う問題

proto-driven-ddd で生成される Entity は ORM タグ(GORM / Drizzle 等)を持ち、テーブル定義の真実を所有する。しかし DB スキーマの **適用** には別の SQL ファイルが必要であり、その差分を手で書くと:

- entity を変えるたびに手書き SQL を整合させる手間が発生
- `CREATE` / `ALTER` / `DROP` の判定が抜けやすい
- 開発者間で migration 漏れが起きる

`mss-migration-gen` はこの差分を **自動算出 + ファイル出力** することで、entity の変更だけが migration 発生のトリガーになるよう設計する。

## 入出力モデル

```
internal/domain/entity/*.gen.*  (ORM タグ付き、mss-protoc-gen 出力)
       │
       ├─ entity registry (var All / export const All) — 全 entity を 1 箇所に列挙
       ↓
mss-migration-gen
   ├─ 現在の schema を entity から抽出 (reflection / ORM API)
   ├─ migrations/.snapshot.json を読む (前回適用 schema)
   └─ diff 算出
       ↓
   migrations/<YYYYMMDDHHMMSS>_<table>.up.sql   (差分のあった entity ごとに 1 file)
   migrations/.snapshot.json                    (更新後の schema、git commit する)
```

**snapshot を git に commit する**のがこの設計の要。チーム間で schema 履歴が一致し、PR レビューでも diff が見えるため再現性が保証される。

## 入力: entity registry

各言語の `mss-protoc-gen` は、@entity ごとに加えて **registry** を出力する。

| 言語 | ファイル | 形式 |
| --- | --- | --- |
| Go | `internal/domain/entity/registry.gen.go` | `var All = []any{ &User{}, ... }` |
| TypeScript | `src/domain/entity/registry.gen.ts` | `export const ALL = [User, Tour, ...] as const` |

`mss-migration-gen` は `entity.All` を読んで、対象 entity の集合を取得する。entity を増やしても registry は再生成されるので、migration ジェネレータ側は変更不要。

## 出力: migration ファイル

| 項目 | 仕様 |
| --- | --- |
| ファイル名 | `migrations/<YYYYMMDDHHMMSS>_<table>.up.sql` |
| timestamp | 実行時刻 (UTC) |
| 1 entity 1 file | 同じ実行内で複数 entity に変更がある場合、テーブル名昇順に 1 秒ずつ繰り上げる |
| down ファイル | **生成しない**(必要なら `<同 timestamp>_<table>.down.sql` を手書きする) |
| 互換 | [`golang-migrate/migrate`](https://github.com/golang-migrate/migrate) の命名規約 |

## 差分判定ルール

`migrations/.snapshot.json` (前回適用 schema) と現在 entity を比較して以下の SQL を発行する。

| 状態 | 出力 |
| --- | --- |
| snapshot に table 不在、entity に存在 | `CREATE TABLE` |
| 既存 table に新カラム | `ALTER TABLE ... ADD COLUMN` |
| 既存 table からカラム消滅 | `ALTER TABLE ... DROP COLUMN` (WARNING コメント付き) |
| 型 / NOT NULL / unique 変化 | `ALTER COLUMN` / `ADD CONSTRAINT` / `DROP CONSTRAINT` |
| インデックス追加 / 削除 / 内容変化 | `CREATE INDEX` / `DROP INDEX` |
| snapshot に table 存在、entity から消滅 | `DROP TABLE` (WARNING コメント付き) |
| 変化なし | 出力なし(冪等) |

## 規約

### CASCADE / DEFAULT は使わない

- `DROP TABLE` / `DROP COLUMN` で `CASCADE` を付けない
- ORM タグの `default:` (gorm) や `.default(...)` (drizzle) は **無視**
- 結果として「NOT NULL カラムを後付け追加」は **2 段階**(NULL 許容で追加 → 値埋め → NOT NULL 化)が必要になる
- 生成 SQL に `-- WARNING:` コメントが入るので、そのまま適用する前に必ずレビュー

### 危険操作は WARNING で警告

| 操作 | 理由 | 警告内容 |
| --- | --- | --- |
| NOT NULL 追加 | 既存行があれば失敗 | 「先に値を埋めてください」 |
| DROP COLUMN | データ消失 | 「CASCADE 不使用なので依存があれば失敗」 |
| DROP TABLE | データ消失 | 同上 |
| 型変更 | `USING` が要るケース | 「失敗時はコメントアウト + 手動移行」 |
| PRIMARY KEY 変更 | 一般に手動対応必須 | 「本ファイルを参考に手書きで書き直し」 |

### snapshot は git commit、SQL を手書き修正したら snapshot も同期

`migrations/.snapshot.json` は **次回 diff の基準**。SQL を手で直したのに snapshot を更新しないと、次回実行で同じ差分が再出現する。手書き修正の後は対応する snapshot 状態へ手で書き戻す(または `mss-migration-gen` を一度走らせて空 diff になることを確認)。

## 適用フロー

1. proto を変更 → `make proto-gen` で entity 再生成
2. `make migration-gen` (内部で `cmd/mss-migration-gen` 実行)
3. 出力 SQL の WARNING を確認、必要なら手動分割
4. SQL + `.snapshot.json` を git commit
5. デプロイ前に `golang-migrate` 等で適用:
   ```
   migrate -path migrations -database "$DATABASE_URL" up
   ```

## 言語実装の指針

`mss-protoc-gen` と同じく、言語別に等価な実装を用意する。

### Go (`cmd/mss-migration-gen/`)

- 入力: `internal/domain/entity` を import + `entity.All` を反復
- schema 抽出: `gorm.io/gorm/schema.Parse` で struct + tag → スキーマメタ
- diff: スナップショット JSON との比較
- SQL: Postgres DDL を `text/template` または直接文字列構築

### TypeScript (Hono / Next 等)

Drizzle ORM を採用しているプロジェクトでは、**`drizzle-kit generate`** を migration ツールとしてそのまま使うのが第一選択。Drizzle の `pgTable` は entity ファイル内に同梱されているので、`drizzle-kit` の `schema` を `src/domain/entity/*.gen.ts` に向ければ等価な機能が得られる。

`mss-migration-gen` を独自実装する場合の方針:

- 入力: `entity/registry.gen.ts` の `ALL` 配列を import
- schema 抽出: Drizzle の `getTableConfig(table)` でカラム / インデックス情報を取得
- diff / SQL emit: Go 版と同じロジック

ただし drizzle-kit と二重管理になるので、TS では **drizzle-kit を採用、`mss-migration-gen` は Go のみ提供** が推奨。プロジェクト方針の差は各 CLAUDE.md で明記する。

## 既存実装の参照ポイント

codegen の `internal/gogen/runtime/cmd/mss-migration-gen/` が参照実装（`make proto-gen` が生成先の `cmd/mss-migration-gen/*.gen.go` に出力する）。

```
cmd/mss-migration-gen/
├── main.go        # CLI (-dir / -dry-run / -name) + 全体フロー
├── schema.go      # gorm/schema → 中間表現
├── snapshot.go    # migrations/.snapshot.json の I/O
├── diff.go        # 中間表現の diff 算出
└── sql.go         # Postgres DDL 出力 + 警告コメント
```

## 拡張の方向性

必要になったら以下のような機能拡張を検討:

- **複合主キー / FK**: 現状は単一カラム PK のみ対応。複数 PK / FK は entity の特殊タグ + diff ロジック拡張が必要
- **Index 命名規則の柔軟化**: 現状は ORM 既定。別命名にしたい場合は entity 側の name タグを尊重
- **down 自動生成**: 危険なので原則手書きだが、CREATE → DROP のような単純なケースを支援するヘルパー
- **複数 DB (MySQL / SQLite)**: SQL emit 部を dialect 抽象化

## 関連パターン

- [proto-driven-ddd.md](./proto-driven-ddd.md) — Entity の出所
- [mss-protoc-gen.md](./mss-protoc-gen.md) — Entity を生成する側のジェネレータ
- [infra-swap.md](./infra-swap.md) — 別 DB に移管する場合の方針(SQL emit 部を dialect 別に書き換える)
