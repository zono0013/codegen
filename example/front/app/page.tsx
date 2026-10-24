import Link from 'next/link'
import { FileTextIcon, UsersIcon } from 'lucide-react'
import { Card, CardDescription, CardHeader, CardTitle } from '@/shared/ui/card'

export default function Home() {
  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-8 p-6">
      <header className="flex flex-col gap-2">
        <h1 className="text-2xl font-semibold tracking-tight">記事投稿アプリ</h1>
        <p className="text-muted-foreground">
          proto 駆動テンプレートのサンプル実装。proto を書き換えて自分のドメインに差し替える。
        </p>
      </header>

      <div className="grid gap-4 sm:grid-cols-2">
        <Link href="/articles">
          <Card className="h-full transition-colors hover:border-ring">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <FileTextIcon className="size-4" />
                記事
              </CardTitle>
              <CardDescription>下書きの作成・編集・公開の切り替え</CardDescription>
            </CardHeader>
          </Card>
        </Link>
        <Link href="/users">
          <Card className="h-full transition-colors hover:border-ring">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <UsersIcon className="size-4" />
                投稿者
              </CardTitle>
              <CardDescription>記事を書くユーザーの管理</CardDescription>
            </CardHeader>
          </Card>
        </Link>
      </div>
    </div>
  )
}
