import { createElement } from 'react'
import type { ReactNode } from 'react'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { listArticles } from '@/generate/api/article.gen'
import type { Article } from '@/generate/schemas/article.gen'
import { useArticleList } from './useArticleList'

vi.mock('@/generate/api/article.gen')

const mockArticle: Article = {
  id: 'a1',
  slug: 'hello-world',
  title: 'はじめての記事',
  body: '本文です',
  author_id: 'u1',
  published: false,
  created_at_unix: 1700000000,
  updated_at_unix: 1700000000,
}

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  function Wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: queryClient }, children)
  }
  return Wrapper
}

describe('useArticleList', () => {
  beforeEach(() => vi.clearAllMocks())

  it('レスポンスの配列をそのまま articles として返す', async () => {
    vi.mocked(listArticles).mockResolvedValue([mockArticle])

    const { result } = renderHook(() => useArticleList(), { wrapper: createWrapper() })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.articles).toEqual([mockArticle])
    expect(result.current.error).toBeNull()
  })

  it('取得前・失敗時は空配列を返す', async () => {
    vi.mocked(listArticles).mockRejectedValue({
      title: 'Internal Server Error',
      detail: '取得に失敗しました',
      status: 500,
    })

    const { result } = renderHook(() => useArticleList(), { wrapper: createWrapper() })

    expect(result.current.articles).toEqual([])
    await waitFor(() => expect(result.current.error).toBe('取得に失敗しました'))
    expect(result.current.articles).toEqual([])
  })
})
