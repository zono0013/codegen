'use client'
import Link from 'next/link'
import { AlertCircleIcon, PlusIcon } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '@/shared/ui/alert'
import { Button } from '@/shared/ui/button'
import { useArticleList, ArticleTable } from '@/features/list-articles'
import { useUserList } from '@/features/list-users'
import { useDeleteArticle } from '@/features/delete-article'

export default function ArticlesPage() {
  const { articles, isLoading, error, refetch } = useArticleList()
  const { users } = useUserList()
  const { remove, isLoading: isDeleting } = useDeleteArticle()

  // feature 同士は import し合わないので、投稿者名の解決は app 層で行う
  const authorNameById = Object.fromEntries(users.map((u) => [u.id, u.name]))

  const handleDelete = async (id: string) => {
    const ok = await remove(id)
    if (ok) refetch()
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-semibold">記事</h1>
        <Button render={<Link href="/articles/new" />}>
          <PlusIcon data-icon="inline-start" />
          記事を書く
        </Button>
      </div>
      {error && (
        <Alert variant="destructive">
          <AlertCircleIcon />
          <AlertTitle>データ取得エラー</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <ArticleTable
        articles={articles}
        authorNameById={authorNameById}
        isLoading={isLoading}
        onDelete={handleDelete}
        isDeleting={isDeleting}
      />
    </div>
  )
}
