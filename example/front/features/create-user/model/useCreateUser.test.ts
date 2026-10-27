import { createElement } from 'react'
import type { ReactNode } from 'react'
import { renderHook, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createUser } from '@/generate/api/user.gen'
import type { User } from '@/generate/schemas/user.gen'
import { useCreateUser } from './useCreateUser'

vi.mock('@/generate/api/user.gen')

const mockUser: User = {
  id: '1',
  name: '田中 太郎',
  email: 'tanaka@example.com',
  created_at: '2023-11-14T22:13:20Z',
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

describe('useCreateUser', () => {
  beforeEach(() => vi.clearAllMocks())

  it('成功時に作成した user を返す', async () => {
    vi.mocked(createUser).mockResolvedValue(mockUser)

    const { result } = renderHook(() => useCreateUser(), { wrapper: createWrapper() })

    let returned: User | null = null
    await act(async () => {
      returned = await result.current.create({ name: '田中 太郎', email: 'tanaka@example.com' })
    })

    expect(returned).toEqual(mockUser)
    expect(result.current.error).toBeNull()
    expect(result.current.isLoading).toBe(false)
  })

  it('API エラー時に null を返して error state をセットする', async () => {
    vi.mocked(createUser).mockRejectedValue(new Error('作成に失敗しました'))

    const { result } = renderHook(() => useCreateUser(), { wrapper: createWrapper() })

    let returned: User | null = null
    await act(async () => {
      returned = await result.current.create({ name: '田中 太郎', email: 'tanaka@example.com' })
    })

    expect(returned).toBeNull()
    expect(result.current.error).toBe('作成に失敗しました')
    expect(result.current.isLoading).toBe(false)
  })

  it('実行中は isLoading が true になる', async () => {
    let resolve: (value: User) => void
    vi.mocked(createUser).mockReturnValue(new Promise(r => { resolve = r }))

    const { result } = renderHook(() => useCreateUser(), { wrapper: createWrapper() })

    act(() => {
      result.current.create({ name: '田中 太郎', email: 'tanaka@example.com' })
    })

    expect(result.current.isLoading).toBe(true)

    await act(async () => { resolve!(mockUser) })

    expect(result.current.isLoading).toBe(false)
  })

  it('RFC 9457 Problem Details のエラーは detail を表示する', async () => {
    vi.mocked(createUser).mockRejectedValue({
      title: 'Conflict',
      detail: 'このメールアドレスは既に使用されています',
      status: 409,
    })

    const { result } = renderHook(() => useCreateUser(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.create({ name: '田中 太郎', email: 'tanaka@example.com' })
    })

    expect(result.current.error).toBe('このメールアドレスは既に使用されています')
  })
})
