'use client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { publishArticle, unpublishArticle } from '@/generate/api/article.gen'
import type { Article } from '@/generate/schemas/article.gen'
import { getErrorMessage } from '@/shared/api/errorUtils'

interface SetPublishedInput {
  id: string
  published: boolean
}

// 公開 / 非公開は別 endpoint だが FE から見れば 1 つのトグル操作なので 1 hook にまとめる。
export function usePublishArticle() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: ({ id, published }: SetPublishedInput) =>
      published ? publishArticle({ id }) : unpublishArticle({ id }),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['article', variables.id] })
      queryClient.invalidateQueries({ queryKey: ['articles'] })
      toast.success(variables.published ? '記事を公開しました' : '記事を下書きに戻しました')
    },
    onError: (e) => {
      toast.error(getErrorMessage(e))
    },
  })

  const setPublished = async (input: SetPublishedInput): Promise<Article | null> => {
    try {
      return await mutation.mutateAsync(input)
    } catch {
      return null
    }
  }

  return {
    setPublished,
    isLoading: mutation.isPending,
  }
}
