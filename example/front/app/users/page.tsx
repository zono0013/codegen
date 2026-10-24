'use client'
import { AlertCircleIcon } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/shared/ui/alert'
import { useUserList, UserTable } from '@/features/list-users'
import { CreateUserDialog } from '@/features/create-user'
import { useDeleteUser } from '@/features/delete-user'

export default function UsersPage() {
  const { users, isLoading, error, refetch } = useUserList()
  const { remove, isLoading: isDeleting } = useDeleteUser()

  const handleDelete = async (id: string) => {
    const ok = await remove(id)
    if (ok) refetch()
  }

  return (
    <div className="p-6 flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">ユーザー管理</h1>
        <CreateUserDialog onSuccess={refetch} />
      </div>
      {error && (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>データ取得エラー</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <UserTable
        users={users}
        isLoading={isLoading}
        onDelete={handleDelete}
        isDeleting={isDeleting}
      />
    </div>
  )
}
