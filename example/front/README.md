# front

[codegen](../../README.md) で proto から API クライアントを生成するプロジェクトのフロントエンド。Next.js 16 (App Router) + React 19 + TypeScript + Tailwind v4 + shadcn/ui。構成は [FSD](https://feature-sliced.design)。

## 起動

```bash
pnpm install
pnpm dev          # http://localhost:3000
```

API（`/api/*`）は Next.js がバックエンドに転送する（`next.config.ts` の `rewrites`）。転送先は `BACKEND_URL`（既定 `http://localhost:8080`、`next build` のときに決まる）。ブラウザから見て同一オリジンなので、バックエンドに CORS の設定は要らない。

## コマンド

| コマンド | 内容 |
| --- | --- |
| `pnpm dev` | 開発サーバー |
| `pnpm build` | プロダクションビルド |
| `pnpm gen` | `../proto` から Zod スキーマ + API クライアントを生成（`generate/` は編集禁止） |
| `pnpm test` | Vitest（watch） |
| `pnpm vitest run` | Vitest（1 回だけ） |
| `pnpm lint` | ESLint |
| `pnpm typecheck` | `tsc --noEmit` |

## ディレクトリ

```
front/
├── app/          # ルーティング + ページ固有コード
├── widgets/      # features の合成ブロック（複数ページで再利用するときだけ）
├── features/     # 機能単位のロジック・UI（feature 同士は import しない）
├── shared/       # 汎用 UI / API クライアント / ユーティリティ
└── generate/     # proto から自動生成（編集禁止）
```

詳細は [`docs/architecture.md`](docs/architecture.md)、規約は [`CLAUDE.md`](CLAUDE.md)。
