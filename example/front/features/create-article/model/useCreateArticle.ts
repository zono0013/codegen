'use client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createArticle } from '@/generate/api/article.gen'
import type { Article, CreateArticleRequest } from '@/generate/schemas/article.gen'
import { getErrorMessage } from '@/shared/api/errorUtils'

export function useCreateArticle() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: (input: CreateArticleRequest) => createArticle(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['articles'] })
    },
  })

  const create = async (input: CreateArticleRequest): Promise<Article | null> => {
    try {
      return await mutation.mutateAsync(input)
    } catch {
      return null
    }
  }

  return {
    create,
    isLoading: mutation.isPending,
    error: mutation.error != null ? getErrorMessage(mutation.error) : null,
  }
}
