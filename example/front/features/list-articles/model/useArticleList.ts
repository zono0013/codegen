'use client'
import { useQuery } from '@tanstack/react-query'
import { listArticles } from '@/generate/api/article.gen'
import type { Article } from '@/generate/schemas/article.gen'
import { getErrorMessage } from '@/shared/api/errorUtils'

export function useArticleList() {
  const query = useQuery<Article[], unknown>({
    queryKey: ['articles'],
    queryFn: () => listArticles(),
  })

  return {
    articles: query.data ?? [],
    isLoading: query.isLoading,
    error: query.error != null ? getErrorMessage(query.error) : null,
    refetch: query.refetch,
  }
}
