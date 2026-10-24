import { createElement } from 'react'
import type { ReactNode } from 'react'
import { renderHook, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createArticle } from '@/generate/api/article.gen'
import type { Article } from '@/generate/schemas/article.gen'
import { useCreateArticle } from './useCreateArticle'

vi.mock('@/generate/api/article.gen')

const created: Article = {
  id: 'a1',
  slug: 'hello-world',
  title: 'はじめての記事',
  body: '本文です',
  author_id: 'u1',
  published: false,
  created_at_unix: 1700000000,
  updated_at_unix: 1700000000,
}

const input = {
  slug: 'hello-world',
  title: 'はじめての記事',
  body: '本文です',
  author_id: 'u1',
}

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  function Wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: queryClient }, children)
  }
  return Wrapper
}

describe('useCreateArticle', () => {
  beforeEach(() => vi.clearAllMocks())

  it('作成した記事は下書きとして返る', async () => {
    vi.mocked(createArticle).mockResolvedValue(created)

    const { result } = renderHook(() => useCreateArticle(), { wrapper: createWrapper() })

    let returned: Article | null = null
    await act(async () => {
      returned = await result.current.create(input)
    })

    expect(returned).toEqual(created)
    expect(returned!.published).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('slug 重複（409）は detail を error に載せて null を返す', async () => {
    vi.mocked(createArticle).mockRejectedValue({
      title: 'Conflict',
      detail: 'article already exists',
      status: 409,
    })

    const { result } = renderHook(() => useCreateArticle(), { wrapper: createWrapper() })

    let returned: Article | null = null
    await act(async () => {
      returned = await result.current.create(input)
    })

    expect(returned).toBeNull()
    expect(result.current.error).toBe('article already exists')
  })
})
