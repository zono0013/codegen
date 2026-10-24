'use client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { deleteUser } from '@/generate/api/user.gen'
import { getErrorMessage } from '@/shared/api/errorUtils'

export function useDeleteUser() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: (id: string) => deleteUser({ id }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
      toast.success('ユーザーを削除しました')
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
