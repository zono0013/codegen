import { createElement } from 'react'
import type { ReactNode } from 'react'
import { renderHook, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { getUser } from '@/generate/api/user.gen'
import type { User } from '@/generate/schemas/user.gen'
import { useUser } from './useUser'

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

describe('useUser', () => {
  beforeEach(() => vi.clearAllMocks())

  it('マウント時に指定した id のユーザーをフェッチする', async () => {
    vi.mocked(getUser).mockResolvedValue(mockUser)

    const { result } = renderHook(() => useUser('1'), { wrapper: createWrapper() })

    expect(result.current.isLoading).toBe(true)

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(result.current.user).toEqual(mockUser)
    expect(result.current.error).toBeNull()
    expect(getUser).toHaveBeenCalledWith({ id: '1' })
  })

  it('API エラー時に error state をセットする', async () => {
    vi.mocked(getUser).mockRejectedValue(new Error('ユーザーが見つかりません'))

    const { result } = renderHook(() => useUser('999'), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(result.current.user).toBeNull()
    expect(result.current.error).toBe('ユーザーが見つかりません')
  })

  it('refetch を呼ぶと再度フェッチする', async () => {
    vi.mocked(getUser).mockResolvedValue(mockUser)

    const { result } = renderHook(() => useUser('1'), { wrapper: createWrapper() })
    await waitFor(() => expect(result.current.isLoading).toBe(false))

    const updatedUser = { ...mockUser, name: '田中 更新' }
    vi.mocked(getUser).mockResolvedValue(updatedUser)

    await act(() => result.current.refetch())

    expect(result.current.user).toEqual(updatedUser)
    expect(getUser).toHaveBeenCalledTimes(2)
  })

  it('id が変わると新しい id でフェッチする', async () => {
    const user1 = { ...mockUser, id: '1' }
    const user2 = { ...mockUser, id: '2', name: '鈴木 花子' }
    vi.mocked(getUser).mockResolvedValueOnce(user1).mockResolvedValueOnce(user2)

    const { result, rerender } = renderHook(({ id }) => useUser(id), {
      initialProps: { id: '1' },
      wrapper: createWrapper(),
    })
    await waitFor(() => expect(result.current.user?.id).toBe('1'))

    rerender({ id: '2' })
    await waitFor(() => expect(result.current.user?.id).toBe('2'))

    expect(getUser).toHaveBeenCalledWith({ id: '2' })
  })

  it('RFC 9457 Problem Details のエラーは detail を表示する', async () => {
    vi.mocked(getUser).mockRejectedValue({
      title: 'Not Found',
      detail: '指定されたユーザーは存在しません',
      status: 404,
    })

    const { result } = renderHook(() => useUser('999'), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    expect(result.current.error).toBe('指定されたユーザーは存在しません')
  })
})
