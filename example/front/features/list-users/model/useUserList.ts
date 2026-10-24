'use client'
import { useQuery } from '@tanstack/react-query'
import { listUsers } from '@/generate/api/user.gen'
import type { User } from '@/generate/schemas/user.gen'
import { getErrorMessage } from '@/shared/api/errorUtils'

export function useUserList() {
  const query = useQuery<User[], unknown>({
    queryKey: ['users'],
    queryFn: async () => {
      return await listUsers()
    },
  })

  return {
    users: query.data ?? [],
    isLoading: query.isLoading,
    error: query.error != null ? getErrorMessage(query.error) : null,
    refetch: query.refetch,
  }
}
