import { createElement } from 'react'
import type { ReactNode } from 'react'
import { renderHook, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { toast } from 'sonner'
import { updateUser } from '@/generate/api/user.gen'
import type { User } from '@/generate/schemas/user.gen'
import { useUpdateUser } from './useUpdateUser'

vi.mock('@/generate/api/user.gen')
vi.mock('sonner')

const mockUser: User = {
  id: '1',
  name: '田中 太郎',
  email: 'tanaka@example.com',
  created_at_unix: 1700000000,
}

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
      mutations: { retry: false },
    },
  })
  function Wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: queryClient }, children)
  }
  return Wrapper
}

describe('useUpdateUser', () => {
  beforeEach(() => vi.clearAllMocks())

  it('成功時に更新した user を返して toast.success を呼ぶ', async () => {
    vi.mocked(updateUser).mockResolvedValue(mockUser)

    const { result } = renderHook(() => useUpdateUser(), { wrapper: createWrapper() })

    let returned: User | null = null
    await act(async () => {
      returned = await result.current.update({ id: '1', name: '田中 太郎', email: 'tanaka@example.com' })
    })

    expect(returned).toEqual(mockUser)
    expect(toast.success).toHaveBeenCalledWith('ユーザー情報を更新しました')
    expect(result.current.error).toBeNull()
    expect(result.current.isLoading).toBe(false)
  })

  it('API エラー時に null を返して error state をセットする', async () => {
    vi.mocked(updateUser).mockRejectedValue(new Error('更新に失敗しました'))

    const { result } = renderHook(() => useUpdateUser(), { wrapper: createWrapper() })

    let returned: User | null = null
    await act(async () => {
      returned = await result.current.update({ id: '1', name: '田中 太郎', email: 'tanaka@example.com' })
    })

    expect(returned).toBeNull()
    expect(toast.success).not.toHaveBeenCalled()
    expect(result.current.error).toBe('更新に失敗しました')
    expect(result.current.isLoading).toBe(false)
  })

  it('実行中は isLoading が true になる', async () => {
    let resolve: (value: User) => void
    vi.mocked(updateUser).mockReturnValue(new Promise(r => { resolve = r }))

    const { result } = renderHook(() => useUpdateUser(), { wrapper: createWrapper() })

    act(() => {
      result.current.update({ id: '1', name: '田中 太郎', email: 'tanaka@example.com' })
    })

    expect(result.current.isLoading).toBe(true)

    await act(async () => { resolve!(mockUser) })

    expect(result.current.isLoading).toBe(false)
  })

  it('RFC 7807 Problem Details のエラーは detail を表示する', async () => {
    vi.mocked(updateUser).mockRejectedValue({
      title: 'Conflict',
      detail: 'このメールアドレスは既に使用されています',
      status: 409,
    })

    const { result } = renderHook(() => useUpdateUser(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.update({ id: '1', name: '田中 太郎', email: 'new@example.com' })
    })

    expect(result.current.error).toBe('このメールアドレスは既に使用されています')
  })
})
