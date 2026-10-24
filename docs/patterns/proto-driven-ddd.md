# Proto 駆動 DDD パターン

バックエンドを「proto を唯一のソース」にして DDD レイヤー全体を自動生成するパターン。言語・フレームワーク非依存。

## 要点

1. **proto (.proto) が真実のソース** — ドメインスキーマも API 境界もすべて `.proto` に書く
2. **proto の変更 → 各層の再生成 → コンパイルエラーで影響範囲検知** の流れで安全に進化させる
3. **Entity / Repository interface / Postgres 実装 / Mock / Usecase interface / Handler / DI 配線は手書きしない** — すべて proto から自動生成
4. **手書きは Service と Usecase 実装のみ**（DI のワイヤリング数行を含む）
5. **永続化は Postgres 単一実装**。ユニットテストは Mock 経由、実 DB は testcontainers 等で別途構築

## レイヤー構成

```
proto/**/*.proto                       ← 唯一の手書き source
   │
   └─ buf generate
      │
      ├→ メッセージ型 / RPC                       （protoc-gen-<lang>, connect-<lang>）
      └→ mss-protoc-gen で以下を生成:
         - entity / repository interface / mock / postgres 実装
         - usecase interface + Input 型
         - handler (Connect for Go/Hono、REST for Next)
         - di 配線（Handlers struct / handler-registry）
                                             │
Handler                ←──────────────────── 生成
  ↓
Usecase interface      ←──────────────────── 生成
  ↓ 実装
<Name>UsecaseImpl                            ← 手書き（残る手書き 1/2）
  ↓
Service                                      ← 手書き（残る手書き 2/2）
  ↓
Repository(interface)  ←──────────────────── 生成
  ↑ 実装
Postgres Repository (GORM / Drizzle) ←────── 生成
  ↓
Entity                 ←──────────────────── 生成（`Hydrate` ファクトリ同梱）
```

## なぜ有効か

### メリット

- **手書きコード量が激減** — User 1 つにつき Entity / Repository interface / Mock / Postgres(ORM) 実装の 4 ファイル（Go の場合 200+ 行）が 0 手書きになる
- **スキーマ変更時の影響が型で検知** — proto を変えると生成物が変わり、それを使う手書き層がコンパイルエラーになる
- **フィールド追加・リネームが安全** — 再生成 → ビルドエラー修正、で漏れがない
- **DB エンジン切り替えが容易** — テンプレート (`.tpl`) を差し替える単位で、Postgres → MySQL / Redis / MongoDB に変更可能（詳細は [infra-swap.md](./infra-swap.md)）

### トレードオフ

- 独自 protoc プラグインの保守コストが発生
- 生成ルールに収まらない Entity は別途手書きする必要がある（が、その時点で proto 駆動に合わない設計と判断すべき）
- マーカー（`@entity` 等）を proto に書くため、proto の読み手に独自規約の知識が必要

## 採用判断

**合う**:
- CRUD 中心のリソース指向 API
- 同じパターンの Entity が多数（数十〜）ある
- ドメイン層の骨格が安定していて、主に追加・拡張が多い

**合わない**:
- Entity が少数で構造がそれぞれ大きく異なる
- ドメインロジックが Entity 自体に多く、生成コードに収まらない振る舞いが多い
- proto を採用していない / 導入できない通信層（REST しか使えない等）

## proto アノテーション（mss-protoc-gen が解釈）

> スタック横断の概念表。このリポジトリでの正確な仕様は [`docs/spec.md`](../spec.md)。

| マーカー | 位置 | 効果 |
| --- | --- | --- |
| `@entity` | message | Entity / Repository interface / Mock / Postgres(GORM / Drizzle) 実装を生成 |
| `@pk` | field | 主キー。`SelectByPK` / `Delete` を生成 |
| `@unique` | field | `SelectBy<Field>` を追加生成 |
| `@email` | field | email 形式バリデーション |
| `@required` | field | 非空バリデーション |
| `@timestamp` | int64 field | 時刻型（`time.Time` / `Date`）にマップ、`_unix` サフィックスを除去 |
| `@http METHOD /path` | rpc | Next の REST Route Handler 生成で使用（`{id}` を Next の `[id]` に変換）。Go/Hono（Connect）では無視 |

```proto
// @entity
message User {
  // @pk
  string id = 1;
  // @unique @email
  string email = 2;
  // @required
  string name = 3;
  // @timestamp
  int64 created_at_unix = 4;
}

service UserService {
  // @http POST /api/users
  rpc CreateUser(CreateUserRequest) returns (CreateUserResponse);

  // @http GET /api/users/{id}
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
}
```

## 関連

- [mss-protoc-gen](./mss-protoc-gen.md) — 生成プラグインそのものの実装ガイド
- [infra-swap.md](./infra-swap.md) — Postgres → MySQL / Redis / MongoDB への切り替え方
