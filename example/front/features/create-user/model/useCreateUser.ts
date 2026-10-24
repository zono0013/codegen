'use client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createUser } from '@/generate/api/user.gen'
import type { CreateUserRequest, User } from '@/generate/schemas/user.gen'
import { getErrorMessage } from '@/shared/api/errorUtils'

export function useCreateUser() {
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: (input: CreateUserRequest) => createUser(input),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['users'] })
    },
  })

  const create = async (input: CreateUserRequest): Promise<User | null> => {
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
