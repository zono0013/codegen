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
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'
import { Skeleton } from '@/shared/ui/skeleton'
import { useArticle } from '@/features/get-article'
import { UpdateArticleForm } from '@/features/update-article'
import { PublishToggle } from '@/features/publish-article'
import { useDeleteArticle } from '@/features/delete-article'

export default function ArticleDetailPage() {
  const { id } = useParams<{ id: string }>()
  const router = useRouter()
  const { article, isLoading, error, refetch } = useArticle(id)
  const { remove, isLoading: isDeleting } = useDeleteArticle()

  const handleDelete = async () => {
    const ok = await remove(id)
    if (ok) router.push('/articles')
  }

  return (
    <div className="flex max-w-2xl flex-col gap-6 p-6">
      <div className="flex items-center gap-3">
        <Button variant="ghost" size="icon-sm" onClick={() => router.push('/articles')}>
          <ArrowLeftIcon />
          <span className="sr-only">一覧に戻る</span>
        </Button>
        <h1 className="text-xl font-semibold">記事を編集</h1>
        <div className="ml-auto">
          <AlertDialog>
            <AlertDialogTrigger
              render={<Button variant="destructive" size="sm" disabled={isDeleting || isLoading} />}
            >
              <TrashIcon data-icon="inline-start" />
              削除
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>記事を削除しますか？</AlertDialogTitle>
                <AlertDialogDescription>
                  {article ? `「${article.title}」` : 'この記事'}を削除します。この操作は元に戻せません。
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

      {article && (
        <PublishToggle id={article.id} published={article.published} onSuccess={refetch} />
      )}

      <Card>
        <CardHeader>
          <CardTitle>本文</CardTitle>
          {article && (
            <CardDescription>
              作成日: {new Date(article.created_at_unix * 1000).toLocaleDateString('ja-JP')} / 更新日:{' '}
              {new Date(article.updated_at_unix * 1000).toLocaleDateString('ja-JP')}
            </CardDescription>
          )}
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="flex flex-col gap-4">
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-10 w-full" />
              <Skeleton className="h-40 w-full" />
            </div>
          ) : article ? (
            <UpdateArticleForm article={article} onSuccess={refetch} />
          ) : null}
        </CardContent>
      </Card>
    </div>
  )
}
