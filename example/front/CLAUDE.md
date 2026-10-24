@AGENTS.md

# Front

[codegen](../../README.md) で proto から API クライアントを生成するプロジェクトのフロントエンド。Next.js (App Router) + TypeScript + Tailwind CSS + pnpm。

API は `../proto/**/*.proto` から生成される（`pnpm gen` = codegen の `make proto-gen`）。サンプル実装は記事投稿アプリ（`Article` + 投稿者 `User`）。

## BE レスポンスの形

BE の REST Handler は proto の Response メッセージをそのまま返さず「中身」だけを返す。生成される型もそれに合わせてある。

| Response の形 | 実レスポンス | 生成される型 |
|---|---|---|
| フィールド 0 個 | 204 No Content | `void` |
| `@entity` 1 個 | そのオブジェクト | `Article` |
| `repeated @entity` 1 個 | 配列 | `Article[]` |
| それ以外 | 全フィールドのオブジェクト | そのまま |

つまり `listArticles()` は `Article[]` を直接返す（`res.articles` ではない）。

## ナレッジ

実装の参照ドキュメントは `docs/` にある。

| ドキュメント | 内容 |
|---|---|
| @docs/architecture.md | FSD レイヤー構成・依存ルール |
| @docs/coding-conventions.md | プロジェクト固有規約 |
| @docs/design-tokens.md | カラーパレット・タイポグラフィ |

## コミット・PR

- **ブランチ**: `feat/<Issue番号>-<topic>` / `fix/<Issue番号>-<topic>`（例: `feat/42-add-login`）
- **メッセージ**: Conventional Commits（`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`）
- **件名言語**: 英語 or 日本語（プロジェクトで統一）
