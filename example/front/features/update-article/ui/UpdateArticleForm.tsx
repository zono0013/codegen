'use client'
import { useState } from 'react'
import { Loader2Icon } from 'lucide-react'
import { Button } from '@/shared/ui/button'
import { Input } from '@/shared/ui/input'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/shared/ui/field'
import type { Article } from '@/generate/schemas/article.gen'
import { useUpdateArticle } from '@/features/update-article/model/useUpdateArticle'

interface UpdateArticleFormProps {
  article: Article
  onSuccess?: () => void
}

export function UpdateArticleForm({ article, onSuccess }: UpdateArticleFormProps) {
  const [title, setTitle] = useState(article.title)
  const [slug, setSlug] = useState(article.slug)
  const [body, setBody] = useState(article.body)
  const { update, isLoading, error } = useUpdateArticle()

  const isDirty = title !== article.title || slug !== article.slug || body !== article.body

  const handleSubmit = async () => {
    const updated = await update({ id: article.id, title, slug, body })
    if (updated) onSuccess?.()
  }

  return (
    <FieldGroup>
      <Field>
        <FieldLabel htmlFor="article-title">タイトル</FieldLabel>
        <Input id="article-title" value={title} onChange={(e) => setTitle(e.target.value)} />
      </Field>
      <Field>
        <FieldLabel htmlFor="article-slug">slug</FieldLabel>
        <Input id="article-slug" value={slug} onChange={(e) => setSlug(e.target.value)} />
      </Field>
      <Field>
        <FieldLabel htmlFor="article-body">本文</FieldLabel>
        <textarea
          id="article-body"
          value={body}
          onChange={(e) => setBody(e.target.value)}
          rows={12}
          className="w-full rounded-lg border border-input bg-transparent p-3 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        />
      </Field>
      {error && <FieldError>{error}</FieldError>}
      <div>
        <Button onClick={handleSubmit} disabled={isLoading || !isDirty || !title || !slug || !body}>
          {isLoading && <Loader2Icon data-icon="inline-start" className="animate-spin" />}
          保存
        </Button>
      </div>
    </FieldGroup>
  )
}
