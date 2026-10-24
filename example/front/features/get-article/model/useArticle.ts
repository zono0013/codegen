'use client'
import { useQuery } from '@tanstack/react-query'
import { getArticle } from '@/generate/api/article.gen'
import type { Article } from '@/generate/schemas/article.gen'
import { getErrorMessage } from '@/shared/api/errorUtils'

export function useArticle(id: string) {
  const query = useQuery<Article, unknown>({
    queryKey: ['article', id],
    queryFn: () => getArticle({ id }),
  })

  return {
    article: query.data ?? null,
    isLoading: query.isLoading,
    error: query.error != null ? getErrorMessage(query.error) : null,
    refetch: query.refetch,
  }
}
