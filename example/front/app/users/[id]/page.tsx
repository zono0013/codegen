'use client'
import { useParams, useRouter } from 'next/navigation'
import { AlertCircleIcon, ArrowLeftIcon, TrashIcon } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/shared/ui/alert'
import { Button } from '@/shared/ui/button'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/shared/ui/alert-dialog'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/shared/ui/card'
import { Skeleton } from '@/shared/ui/skeleton'
import { useUser } from '@/features/get-user'
import { UpdateUserForm } from '@/features/update-user'
import { useDeleteUser } from '@/features/delete-user'

export default function UserDetailPage() {
  const { id } = useParams<{ id: string }>()
  const router = useRouter()
  const { user, isLoading, error, refetch } = useUser(id)
  const { remove, isLoading: isDeleting } = useDeleteUser()

  const handleDelete = async () => {
    const ok = await remove(id)
    if (ok) router.push('/users')
  }

  return (
    <div className="p-6 flex flex-col gap-6 max-w-xl">
      <div className="flex items-center gap-3">
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => router.push('/users')}
        >
          <ArrowLeftIcon />
          <span className="sr-only">一覧に戻る</span>
        </Button>
        <h1 className="text-xl font-semibold">ユーザー詳細</h1>
        <div className="ml-auto">
          <AlertDialog>
            <AlertDialogTrigger
              render={
                <Button
                  variant="destructive"
                  size="sm"
                  disabled={isDeleting || isLoading}
                />
              }
            >
              <TrashIcon data-icon="inline-start" />
              削除
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>ユーザーを削除しますか？</AlertDialogTitle>
                <AlertDialogDescription>
                  {user
                    ? `${user.name}（${user.email}）`
                    : 'このユーザー'}
                  を削除します。この操作は元に戻せません。
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>キャンセル</AlertDialogCancel>
                <AlertDialogAction variant="destructive" onClick={handleDelete}>
                  削除
                </AlertDialogAction>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>取得エラー</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader>
          <CardTitle>ユーザー情報</CardTitle>
          {user && (
            <CardDescription>
              登録日:{' '}
              {new Date(user.created_at).toLocaleDateString('ja-JP')}
            </CardDescription>
          )}
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="flex flex-col gap-4">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-9 w-16" />
            </div>
          ) : user ? (
            <UpdateUserForm user={user} onSuccess={refetch} />
          ) : null}
        </CardContent>
      </Card>
    </div>
  )
}
