'use client'
import Link from 'next/link'
import { PencilIcon, TrashIcon } from 'lucide-react'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/shared/ui/table'
import { Skeleton } from '@/shared/ui/skeleton'
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
import type { Article } from '@/generate/schemas/article.gen'

interface ArticleTableProps {
  articles: Article[]
  // 投稿者名の解決は上位（app 層）から渡す。feature 同士は import し合わない。
  authorNameById?: Record<string, string>
  isLoading: boolean
  onDelete: (id: string) => void
  isDeleting?: boolean
}

export function ArticleTable({
  articles,
  authorNameById,
  isLoading,
  onDelete,
  isDeleting,
}: ArticleTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>タイトル</TableHead>
          <TableHead>slug</TableHead>
          <TableHead>投稿者</TableHead>
          <TableHead>状態</TableHead>
          <TableHead>更新日</TableHead>
          <TableHead className="w-24" />
        </TableRow>
      </TableHeader>
      <TableBody>
        {isLoading
          ? Array.from({ length: 3 }).map((_, i) => (
              <TableRow key={i}>
                <TableCell><Skeleton className="h-4 w-48" /></TableCell>
                <TableCell><Skeleton className="h-4 w-32" /></TableCell>
                <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                <TableCell><Skeleton className="h-4 w-16" /></TableCell>
                <TableCell><Skeleton className="h-4 w-24" /></TableCell>
                <TableCell />
              </TableRow>
            ))
          : articles.map((article) => (
              <TableRow key={article.id}>
                <TableCell className="font-medium">{article.title}</TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">
                  {article.slug}
                </TableCell>
                <TableCell className="text-muted-foreground">
                  {authorNameById?.[article.author_id] ?? '—'}
                </TableCell>
                <TableCell>
                  <Badge variant={article.published ? 'default' : 'outline'}>
                    {article.published ? '公開' : '下書き'}
                  </Badge>
                </TableCell>
                <TableCell className="font-mono text-xs text-muted-foreground">
                  {new Date(article.updated_at_unix * 1000).toLocaleDateString('ja-JP')}
                </TableCell>
                <TableCell>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      render={<Link href={`/articles/${article.id}`} />}
                    >
                      <PencilIcon />
                      <span className="sr-only">{article.title}を編集</span>
                    </Button>
                    <AlertDialog>
                      <AlertDialogTrigger
                        render={<Button variant="ghost" size="icon-sm" disabled={isDeleting} />}
                      >
                        <TrashIcon />
                        <span className="sr-only">{article.title}を削除</span>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>記事を削除しますか？</AlertDialogTitle>
                          <AlertDialogDescription>
                            「{article.title}」を削除します。この操作は元に戻せません。
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>キャンセル</AlertDialogCancel>
                          <AlertDialogAction
                            variant="destructive"
                            onClick={() => onDelete(article.id)}
                          >
                            削除
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  </div>
                </TableCell>
              </TableRow>
            ))}
      </TableBody>
    </Table>
  )
}
