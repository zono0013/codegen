'use client'
import { useQuery } from '@tanstack/react-query'
import { getUser } from '@/generate/api/user.gen'
import type { User } from '@/generate/schemas/user.gen'
import { getErrorMessage } from '@/shared/api/errorUtils'

export function useUser(id: string) {
  const query = useQuery<User, unknown>({
    queryKey: ['user', id],
    queryFn: async () => {
      return await getUser({ id })
    },
  })

  return {
    user: query.data ?? null,
    isLoading: query.isLoading,
    error: query.error != null ? getErrorMessage(query.error) : null,
    refetch: query.refetch,
  }
}
