# Coding Conventions

このプロジェクト固有のコーディング規約。「プロジェクト内で何をどう書くか」を定める。

---

## 通知・フィードバック

**ユーザーへの通知はすべて `sonner` の `toast()` を使う。**

```ts
import { toast } from 'sonner'

toast.success('ユーザーを追加しました')
toast.error('保存に失敗しました')

const id = toast.loading('保存中...')
// 完了後
toast.dismiss(id)
```

**禁止事項**

- `window.alert()` / `window.confirm()` / `window.prompt()` — 絶対に使わない
- 破壊的操作の確認には `AlertDialog` を使う（`window.confirm` の代替）

---

## API エラーハンドリング

BE は RFC 9457 Problem Details 形式でエラーを返す。`apiClient` は失敗すると `ApiError`（`Error` を継承し、Problem Details の項目を持つ）で reject する。`shared/api/errorUtils` を使って型安全に扱う。

```ts
import { getErrorMessage, isProblem } from '@/shared/api/errorUtils'

try {
  await someApiCall()
} catch (e) {
  // 人間向けのメッセージを取り出す（Problem.detail → Problem.title → e.message の優先順）
  toast.error(getErrorMessage(e))

  // フィールドエラーが必要なときは isProblem で絞り込む
  if (isProblem(e) && e.errors) {
    // e.errors: FieldError[]
  }
}
```

hook 内でエラーを state に持つ場合も同様に `getErrorMessage()` で文字列化する。

---

## 自動生成コード（`generate/`）

`pnpm gen` で proto から生成される。**直接編集しない。**

| ディレクトリ | 内容 | 使い方 |
|---|---|---|
| `generate/api/` | 型付き API 関数 | そのまま呼ぶ |
| `generate/schemas/` | Zod スキーマ + TypeScript 型 | 型として import する |

```ts
// API 呼び出し
import { listUsers, createUser } from '@/generate/api/user.gen'

// 型の利用（snake_case のまま使う。camelCase に変換しない）
import type { User, CreateUserRequest } from '@/generate/schemas/user.gen'
```

変更が必要なら proto を修正して `pnpm gen` を再実行する。

---

## `'use client'` の付け方

**Server Component をデフォルトとする。** `'use client'` は以下のいずれかを使うときだけ付ける。

- `useState` / `useEffect` などの React hooks
- ブラウザ API（`localStorage`、`window` など）
- イベントハンドラを直接持つコンポーネント
- `apiClient`（axios を包んだクライアント）を呼ぶコード — API の呼び出しは TanStack Query の hook（クライアント側）から行う方針のため。`apiClient` は同一オリジンの相対パス（`/api/*`）に送るので、ブラウザでしか動かない（サーバー側から呼ぶならバックエンドの URL を直接使う）

feature の model hook はすべてクライアント専用になるため、hook ファイルの先頭に `'use client'` を付ける。

---

## TypeScript

- `any` は使わない。型が不明なら `unknown` を使い、型ガードで絞り込む
- `as` キャストは外部入力（API レスポンス、localStorage 等）を Zod で parse した後のみ許容
- Zod スキーマが生成済みなら `.parse()` または `.safeParse()` で検証してから使う
- **`undefined` を値として直接使わない。** オプショナルな値は `null` で統一する

```ts
// ❌
const name: string | undefined = undefined

// ✅
const name: string | null = null
```

関数の戻り値で「何もない」を表す場合も `null` を返す（hook の初期値、API が「見つからない」を返す場合など）。
`undefined` が紛れ込みやすいパターン（配列の `.find()` や オプショナルチェーン `?.` の戻り値）は必要に応じて `?? null` で変換する。

---

## コメント

**WHY のみ書く。WHAT は書かない。**

```ts
// ✅ なぜこうするかを書く
// BE が cursor-based pagination を返すため、空文字は「先頭ページ」の意味になる
const after = id !== '' ? id : undefined

// ❌ コードを言葉で繰り返すだけのコメントは書かない
// id が空でなければ after に id をセットする
const after = id !== '' ? id : undefined
```

---

## shadcn UI コンポーネント

既存コンポーネントを優先する。カスタム実装の前に `pnpm dlx shadcn@latest search` で探す。

詳細は [shadcn/ui のドキュメント](https://ui.shadcn.com/docs) を参照。

---

## 認証を入れるとき

`shared/api/apiClient.ts` は認証を持たない（サンプルの API に認証が無いため）。入れるときは、`ApiClient` のコンストラクタで axios のインターセプターを足す。

- リクエスト: `this.axiosInstance.interceptors.request.use(...)` でトークンを `Authorization` ヘッダーに付ける
- レスポンス: 401 のときのトークンの更新やログイン画面への移動は、コンストラクタの中で既存の `interceptors.response.use(...)`（`toApiError` で ApiError に変える）より**先に**登録する。axios のレスポンスのインターセプターは登録した順に動くので、後から足すと AxiosError ではなく ApiError を受け取り、`error.config` を使ったリトライが書けない
- トークンはメモリか HttpOnly Cookie に置き、`localStorage` には置かない（XSS で読まれるため）
- Cookie を使う場合も、API は同一オリジン（`/api/*` を Next.js がバックエンドに転送する）なので、`withCredentials` やバックエンドの CORS の設定は要らない。Cookie は HttpOnly / Secure / SameSite を付けて発行する
