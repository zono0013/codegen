'use client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { deleteArticle } from '@/generate/api/article.gen'
import { getErrorMessage } from '@/shared/api/errorUtils'

export function useDeleteArticle() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: (id: string) => deleteArticle({ id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['articles'] })
      toast.success('記事を削除しました')
    },
    onError: (e) => {
      toast.error(getErrorMessage(e))
    },
  })

  const remove = async (id: string): Promise<boolean> => {
    try {
      await mutation.mutateAsync(id)
      return true
    } catch {
      return false
    }
  }

  return {
    remove,
    isLoading: mutation.isPending,
  }
}
