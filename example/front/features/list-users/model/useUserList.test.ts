import { createElement } from 'react'
import type { ReactNode } from 'react'
import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { listUsers } from '@/generate/api/user.gen'
import type { User } from '@/generate/schemas/user.gen'
import { useUserList } from './useUserList'

vi.mock('@/generate/api/user.gen')

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

describe('useUserList', () => {
  beforeEach(() => vi.clearAllMocks())

  it('マウント時にユーザー一覧をフェッチする', async () => {
    vi.mocked(listUsers).mockResolvedValue([mockUser])

    const { result } = renderHook(() => useUserList(), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(result.current.users).toEqual([mockUser])
    expect(result.current.error).toBeNull()
  })

  it('API エラー時に error state をセットする', async () => {
    vi.mocked(listUsers).mockRejectedValue(new Error('通信エラー'))

    const { result } = renderHook(() => useUserList(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(result.current.users).toEqual([])
    expect(result.current.error).toBe('通信エラー')
  })

  it('refetch を呼ぶと再度フェッチする', async () => {
    vi.mocked(listUsers).mockResolvedValue([mockUser])

    const { result } = renderHook(() => useUserList(), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    vi.mocked(listUsers).mockResolvedValue([])
    await act(() => result.current.refetch())

    expect(result.current.users).toEqual([])
    expect(listUsers).toHaveBeenCalledTimes(2)
  })

  it('RFC 7807 Problem Details のエラーは detail を表示する', async () => {
    vi.mocked(listUsers).mockRejectedValue({
      title: 'Internal Server Error',
      detail: 'データベース接続に失敗しました',
      status: 500,
    })

    const { result } = renderHook(() => useUserList(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(result.current.error).toBe('データベース接続に失敗しました')
  })
})
