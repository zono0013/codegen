'use client'
import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Loader2Icon } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/shared/ui/button'
import { Input } from '@/shared/ui/input'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/shared/ui/field'
import { useCreateUser } from '@/features/create-user/model/useCreateUser'

export function CreateUserForm() {
  const router = useRouter()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const { create, isLoading, error } = useCreateUser()

  const handleSubmit = async () => {
    const user = await create({ name, email })
    if (user) {
      toast.success(`${user.name} を追加しました`)
      router.push('/users')
    }
  }

  return (
    <FieldGroup>
      <Field>
        <FieldLabel htmlFor="new-user-name">名前</FieldLabel>
        <Input
          id="new-user-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="山田 太郎"
        />
      </Field>
      <Field>
        <FieldLabel htmlFor="new-user-email">メールアドレス</FieldLabel>
        <Input
          id="new-user-email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          placeholder="taro@example.com"
        />
      </Field>
      {error && <FieldError>{error}</FieldError>}
      <div className="flex gap-2">
        <Button
          variant="outline"
          onClick={() => router.push('/users')}
          disabled={isLoading}
        >
          キャンセル
        </Button>
        <Button onClick={handleSubmit} disabled={isLoading || !name || !email}>
          {isLoading && <Loader2Icon data-icon="inline-start" className="animate-spin" />}
          追加
        </Button>
      </div>
    </FieldGroup>
  )
}
