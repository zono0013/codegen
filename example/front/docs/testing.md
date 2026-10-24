# Testing

Vitest + React Testing Library。

## コマンド

```bash
pnpm test              # watch モード
pnpm test --run        # 1回実行
pnpm test --run <file> # 特定ファイルのみ
pnpm test:coverage     # カバレッジ
```

## 基本原則

- **AAA パターン**: Arrange（準備）→ Act（操作）→ Assert（確認）
- **1 テスト = 1 つの振る舞い**
- **実装ではなく振る舞いをテストする**（内部 state を直接 assert しない）
- **per test で前提を書く**（`beforeEach` はリセットだけ）

## Hook テスト

```ts
import { renderHook, act } from '@testing-library/react'
import { toast } from 'sonner'
import { updateUser } from '@/generate/api/user.gen'
import { useUpdateUser } from '../useUpdateUser'

vi.mock('@/generate/api/user.gen')
vi.mock('sonner')

describe('useUpdateUser', () => {
  beforeEach(() => vi.clearAllMocks())

  it('成功時は toast.success を呼ぶ', async () => {
    vi.mocked(updateUser).mockResolvedValue({ user: mockUser })

    const { result } = renderHook(() => useUpdateUser())
    await act(() => result.current.update({ id: '1', name: 'new', email: 'a@b.com' }))

    expect(toast.success).toHaveBeenCalled()
    expect(result.current.isLoading).toBe(false)
  })

  it('API エラー時は error state をセットする', async () => {
    vi.mocked(updateUser).mockRejectedValue(new Error('失敗'))

    const { result } = renderHook(() => useUpdateUser())
    await act(() => result.current.update({ id: '1', name: 'new', email: 'a@b.com' }))

    expect(result.current.error).toBe('失敗')
  })
})
```

## コンポーネントテスト

```ts
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { UpdateUserForm } from '../UpdateUserForm'

const mockUser = { id: '1', name: '田中', email: 'tanaka@example.com' }

describe('UpdateUserForm', () => {
  it('変更なしは保存ボタンが disabled', () => {
    render(<UpdateUserForm user={mockUser} />)
    expect(screen.getByRole('button', { name: '保存' })).toBeDisabled()
  })

  it('値を変えると保存ボタンが enabled になる', async () => {
    render(<UpdateUserForm user={mockUser} />)
    await userEvent.clear(screen.getByLabelText('名前'))
    await userEvent.type(screen.getByLabelText('名前'), '新しい名前')
    expect(screen.getByRole('button', { name: '保存' })).toBeEnabled()
  })
})
```

## モックのルール

| 何をモックするか | 方法 |
|---|---|
| `generate/api/` の API 関数 | `vi.mock('@/generate/api/xxx.gen')` |
| `sonner` の toast | `vi.mock('sonner')` |
| `next/navigation` の router | `vi.mock('next/navigation', () => ({ useRouter: () => ({ push: vi.fn() }) }))` |
| shadcn コンポーネント | **モックしない**（実コンポーネントを使う） |

## RFC 9457（Problem Details）エラーのテスト

`apiClient` は失敗すると `ApiError`（`Error` を継承し、Problem Details の項目を持つ）で reject する。hook テストでは同じ形でモックする:

```ts
import { ApiError } from '@/shared/api/errorUtils'

vi.mocked(createUser).mockRejectedValue(
  new ApiError({
    type: 'about:blank',
    title: 'Conflict',
    status: 409,
    detail: 'このメールアドレスは既に使用されています',
  })
)

// hook 側で getErrorMessage(e) → 'このメールアドレスは既に使用されています' になる
expect(result.current.error).toBe('このメールアドレスは既に使用されています')
```

## ファイル配置

```
features/
  update-user/
    model/
      useUpdateUser.ts
      useUpdateUser.test.ts   ← hook テストはモデルの隣
    ui/
      UpdateUserForm.tsx
      UpdateUserForm.test.tsx  ← コンポーネントテストも隣
```
