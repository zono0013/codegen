'use client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { updateArticle } from '@/generate/api/article.gen'
import type { Article, UpdateArticleRequest } from '@/generate/schemas/article.gen'
import { getErrorMessage } from '@/shared/api/errorUtils'

export function useUpdateArticle() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: (input: UpdateArticleRequest) => updateArticle(input),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['article', variables.id] })
      queryClient.invalidateQueries({ queryKey: ['articles'] })
      toast.success('記事を更新しました')
    },
  })

  const update = async (input: UpdateArticleRequest): Promise<Article | null> => {
    try {
      return await mutation.mutateAsync(input)
    } catch {
      return null
    }
  }

  return {
    update,
    isLoading: mutation.isPending,
    error: mutation.error != null ? getErrorMessage(mutation.error) : null,
  }
}
