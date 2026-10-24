'use client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { updateUser } from '@/generate/api/user.gen'
import type { UpdateUserRequest, User } from '@/generate/schemas/user.gen'
import { getErrorMessage } from '@/shared/api/errorUtils'

export function useUpdateUser() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: (input: UpdateUserRequest) => updateUser(input),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['user', variables.id] })
      toast.success('ユーザー情報を更新しました')
    },
  })

  const update = async (input: UpdateUserRequest): Promise<User | null> => {
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
