'use client'
import { useState, useEffect } from 'react'
import { Loader2Icon } from 'lucide-react'
import { Button } from '@/shared/ui/button'
import { Input } from '@/shared/ui/input'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/shared/ui/field'
import type { User } from '@/generate/schemas/user.gen'
import { useUpdateUser } from '@/features/update-user/model/useUpdateUser'

interface UpdateUserFormProps {
  user: User
  onSuccess?: () => void
}

export function UpdateUserForm({ user, onSuccess }: UpdateUserFormProps) {
  const [name, setName] = useState(user.name)
  const [email, setEmail] = useState(user.email)
  const { update, isLoading, error } = useUpdateUser()

  // user.id が変わる（別ユーザーへの遷移）ときだけリセットする
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setName(user.name)
    setEmail(user.email)
  }, [user.id]) // eslint-disable-line react-hooks/exhaustive-deps

  const isDirty = name !== user.name || email !== user.email

  const handleSubmit = async () => {
    const updated = await update({ id: user.id, name, email })
    if (updated) onSuccess?.()
  }

  return (
    <FieldGroup>
      <Field>
        <FieldLabel htmlFor="update-user-name">名前</FieldLabel>
        <Input
          id="update-user-name"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </Field>
      <Field>
        <FieldLabel htmlFor="update-user-email">メールアドレス</FieldLabel>
        <Input
          id="update-user-email"
          type="email"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
        />
      </Field>
      {error && <FieldError>{error}</FieldError>}
      <Button
        onClick={handleSubmit}
        disabled={isLoading || !isDirty || !name || !email}
      >
        {isLoading && <Loader2Icon data-icon="inline-start" className="animate-spin" />}
        保存
      </Button>
    </FieldGroup>
  )
}
