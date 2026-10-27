import { createElement } from 'react'
import type { ReactNode } from 'react'
import { renderHook, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { toast } from 'sonner'
import { publishArticle, unpublishArticle } from '@/generate/api/article.gen'
import type { Article } from '@/generate/schemas/article.gen'
import { usePublishArticle } from './usePublishArticle'

vi.mock('@/generate/api/article.gen')
vi.mock('sonner')

const draft: Article = {
  id: 'a1',
  slug: 'hello-world',
  title: 'はじめての記事',
  body: '本文です',
  author_id: 'u1',
  published: false,
  created_at: '2023-11-14T22:13:20Z',
  updated_at: '2023-11-14T22:13:20Z',
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

describe('usePublishArticle', () => {
  beforeEach(() => vi.clearAllMocks())

  it('published: true なら publish endpoint を叩く', async () => {
    vi.mocked(publishArticle).mockResolvedValue({ ...draft, published: true })

    const { result } = renderHook(() => usePublishArticle(), { wrapper: createWrapper() })

    let returned: Article | null = null
    await act(async () => {
      returned = await result.current.setPublished({ id: 'a1', published: true })
    })

    expect(publishArticle).toHaveBeenCalledWith({ id: 'a1' })
    expect(unpublishArticle).not.toHaveBeenCalled()
    expect(returned).toMatchObject({ published: true })
    expect(toast.success).toHaveBeenCalledWith('記事を公開しました')
  })

  it('published: false なら unpublish endpoint を叩く', async () => {
    vi.mocked(unpublishArticle).mockResolvedValue(draft)

    const { result } = renderHook(() => usePublishArticle(), { wrapper: createWrapper() })

    await act(async () => {
      await result.current.setPublished({ id: 'a1', published: false })
    })

    expect(unpublishArticle).toHaveBeenCalledWith({ id: 'a1' })
    expect(publishArticle).not.toHaveBeenCalled()
    expect(toast.success).toHaveBeenCalledWith('記事を下書きに戻しました')
  })

  it('失敗時は null を返して toast.error を出す', async () => {
    vi.mocked(publishArticle).mockRejectedValue({
      title: 'Not Found',
      detail: 'article not found: a1',
      status: 404,
    })

    const { result } = renderHook(() => usePublishArticle(), { wrapper: createWrapper() })

    let returned: Article | null = null
    await act(async () => {
      returned = await result.current.setPublished({ id: 'a1', published: true })
    })

    expect(returned).toBeNull()
    expect(toast.error).toHaveBeenCalledWith('article not found: a1')
  })
})
