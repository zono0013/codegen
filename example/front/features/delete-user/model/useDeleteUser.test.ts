import { createElement } from 'react'
import type { ReactNode } from 'react'
import { renderHook, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { toast } from 'sonner'
import { deleteUser } from '@/generate/api/user.gen'
import { useDeleteUser } from './useDeleteUser'

vi.mock('@/generate/api/user.gen')
vi.mock('sonner')

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

describe('useDeleteUser', () => {
  beforeEach(() => vi.clearAllMocks())

  it('成功時に true を返して toast.success を呼ぶ', async () => {
    vi.mocked(deleteUser).mockResolvedValue(undefined)

    const { result } = renderHook(() => useDeleteUser(), { wrapper: createWrapper() })

    let returned: boolean = false
    await act(async () => {
      returned = await result.current.remove('1')
    })

    expect(returned).toBe(true)
    expect(toast.success).toHaveBeenCalledWith('ユーザーを削除しました')
    expect(result.current.isLoading).toBe(false)
  })

  it('API エラー時に false を返して toast.error を呼ぶ', async () => {
    vi.mocked(deleteUser).mockRejectedValue(new Error('削除に失敗しました'))

    const { result } = renderHook(() => useDeleteUser(), { wrapper: createWrapper() })

    let returned: boolean = true
    await act(async () => {
      returned = await result.current.remove('1')
    })

    expect(returned).toBe(false)
    expect(toast.error).toHaveBeenCalledWith('削除に失敗しました')
    expect(result.current.isLoading).toBe(false)
  })

  it('実行中は isLoading が true になる', async () => {
    let resolve: (value: void) => void
    vi.mocked(deleteUser).mockReturnValue(new Promise(r => { resolve = r }))

    const { result } = renderHook(() => useDeleteUser(), { wrapper: createWrapper() })

    act(() => {
      result.current.remove('1')
    })

    expect(result.current.isLoading).toBe(true)

    await act(async () => { resolve!(undefined) })

    expect(result.current.isLoading).toBe(false)
  })
})
