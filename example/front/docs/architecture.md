# Architecture

## レイヤー構成（FSD ベース）

公式 FSD（https://feature-sliced.design）をベースとする。**独自レイヤー・独自セグメントは原則追加しない**。

```
front/
├── app/          # Next.js ルーティング + ページ固有コード
├── widgets/      # features の合成ブロック（複数ページで再利用する場合のみ）
├── features/     # 機能単位のロジック・UI（複数ドメインをまたいでよい）
├── shared/       # 汎用 UI・ユーティリティ・API クライアント（ビジネスロジックなし）
└── generate/     # proto から自動生成（編集禁止）
```

### 依存方向

```
app → widgets → features → shared
```

`generate/` はどのレイヤーからも import 可能。`generate/` 自身は他レイヤーを import しない。

**同レイヤー内の横断 import ルール（FSD 公式）**

| レイヤー | 同レイヤー間の横断 import |
|---|---|
| `features` | **禁止**。feature 同士は import し合わない。連携は上位（`app/` or `widgets/`）からコールバックで渡す |
| `shared` | **許可**。`shared` はスライスを持たない例外レイヤー。セグメント間（`shared/lib/` → `shared/api/` 等）は自由に import できる |
| `widgets` | **禁止**。widget 同士は import し合わない |

> FSD 公式: *"Layers App and Shared are exceptions… segments can import each other freely."*

## 各レイヤーの責務

**`shared/`**
- ビジネスロジックを持たない純粋な汎用コード
- `ui/` — Button、Input、Modal など汎用 UI コンポーネント
- `api/` — axios インスタンス・インターセプター設定
- `lib/` — 日付フォーマット・数値変換などのユーティリティ
- `config/` — 環境変数・定数

**`features/`**
- ユーザーが行う操作の単位で切る（ドメインではなく機能）
- 複数ドメインをまたいでよい（JOIN データの処理はここ）
- 例: `features/filter-projects/`、`features/assign-staff/`、`features/update-project-status/`
- **feature 同士は import し合わない。** feature をまたぐ連携はコールバックを `app/` か `widgets/` から渡す

**`widgets/`**
- デフォルトは空。features の合成 UI が複数ページで必要になったときだけ作成する
- 新しい widget を作る前に「本当に複数ページで使うか」を確認すること

**`app/`**
- Next.js のルーティング（App Router）
- ページ固有のコード（他ページで再利用しないもの）は `_components/`、`_hooks/` に置く
- グローバルプロバイダー・レイアウト・スタイル

## セグメント

公式 FSD セグメントを使う。独自セグメントは原則追加しない。

| セグメント | 内容 |
|-----------|------|
| `ui/` | React コンポーネント |
| `api/` | API 呼び出し関数 |
| `model/` | 型定義・状態管理・ビジネスロジック |
| `lib/` | ヘルパー関数（フレームワーク非依存） |
| `config/` | 定数・設定値 |

`hooks/` を追加したくなったら、`model/` または `lib/` に収まらないか先に検討すること。

## 自動生成コード（`generate/`）

`pnpm gen`（= codegen の `make proto-gen`）で proto から生成される。ジェネレータ本体は codegen（`../../../`）の `internal/tsgen/` にあり、BE・OpenAPI と同じ proto 解釈を共有する（仕様は `../../../docs/spec.md`）。

- `generate/schemas/` — Zod スキーマ + TypeScript 型（snake_case のまま、変換しない）
- `generate/api/` — 型付き API 関数（`shared/api/apiClient` を使用）

`generate/` 配下は編集禁止。変更が必要なら proto を修正して再生成する。

## コーディング規約

- コメントは WHY のみ。WHAT は書かない
- `any` は使わない
- `"use client"` は必要な箇所のみ（Server Component をデフォルトとする）
- API レスポンスは `generate/schemas/` の Zod スキーマで parse してから使う
- API の型は snake_case のまま使う（camelCase への変換はしない）
