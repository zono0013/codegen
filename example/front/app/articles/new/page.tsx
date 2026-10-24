'use client'
import { useRouter } from 'next/navigation'
import { AlertCircleIcon, ArrowLeftIcon } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/shared/ui/alert'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/shared/ui/card'
import { CreateArticleForm } from '@/features/create-article'
import { useUserList } from '@/features/list-users'

export default function NewArticlePage() {
  const router = useRouter()
  const { users, isLoading } = useUserList()

  return (
    <div className="flex max-w-2xl flex-col gap-6 p-6">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon-sm" onClick={() => router.push('/articles')}>
          <ArrowLeftIcon />
          <span className="sr-only">一覧に戻る</span>
        </Button>
        <h1 className="text-xl font-semibold">記事を書く</h1>
      </div>

      {!isLoading && users.length === 0 && (
        <Alert>
          <AlertCircleIcon />
          <AlertTitle>投稿者がいません</AlertTitle>
          <AlertDescription>
            先に投稿者を登録してください（記事の author_id は実在する user のみ許可）。
          </AlertDescription>
        </Alert>
      )}

      <Card>
        <CardHeader>
          <CardTitle>新規記事</CardTitle>
        </CardHeader>
        <CardContent>
          <CreateArticleForm authors={users.map((u) => ({ id: u.id, name: u.name }))} />
        </CardContent>
      </Card>
    </div>
  )
}
