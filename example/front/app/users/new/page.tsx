'use client'
import { useRouter } from 'next/navigation'
import { ArrowLeftIcon } from 'lucide-react'
import { Button } from '@/shared/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/shared/ui/card'
import { CreateUserForm } from '@/features/create-user'

export default function NewUserPage() {
  const router = useRouter()

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
        <h1 className="text-xl font-semibold">ユーザーを追加</h1>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>新規ユーザー</CardTitle>
        </CardHeader>
        <CardContent>
          <CreateUserForm />
        </CardContent>
      </Card>
    </div>
  )
}
