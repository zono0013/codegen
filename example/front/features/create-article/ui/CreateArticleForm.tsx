'use client'
import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { Loader2Icon } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/shared/ui/button'
import { Input } from '@/shared/ui/input'
import { Field, FieldError, FieldGroup, FieldLabel } from '@/shared/ui/field'
import { useCreateArticle } from '@/features/create-article/model/useCreateArticle'

export interface AuthorOption {
  id: string
  name: string
}

interface CreateArticleFormProps {
  // 投稿者一覧は app 層が list-users feature から取得して渡す
  authors: AuthorOption[]
}

export function CreateArticleForm({ authors }: CreateArticleFormProps) {
  const router = useRouter()
  const [title, setTitle] = useState('')
  const [slug, setSlug] = useState('')
  const [body, setBody] = useState('')
  const [authorId, setAuthorId] = useState('')
  const { create, isLoading, error } = useCreateArticle()

  const canSubmit = Boolean(title && slug && body && authorId) && !isLoading

  const handleSubmit = async () => {
    const article = await create({ title, slug, body, author_id: authorId })
    if (article) {
      toast.success(`「${article.title}」を下書きとして保存しました`)
      router.push(`/articles/${article.id}`)
    }
  }

  return (
    <FieldGroup>
      <Field>
        <FieldLabel htmlFor="new-article-title">タイトル</FieldLabel>
        <Input
          id="new-article-title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="はじめての記事"
        />
      </Field>
      <Field>
        <FieldLabel htmlFor="new-article-slug">slug</FieldLabel>
        <Input
          id="new-article-slug"
          value={slug}
          onChange={(e) => setSlug(e.target.value)}
          placeholder="hello-world"
        />
      </Field>
      <Field>
        <FieldLabel htmlFor="new-article-author">投稿者</FieldLabel>
        <select
          id="new-article-author"
          value={authorId}
          onChange={(e) => setAuthorId(e.target.value)}
          className="h-9 w-full rounded-lg border border-input bg-transparent px-3 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <option value="">選択してください</option>
          {authors.map((author) => (
            <option key={author.id} value={author.id}>
              {author.name}
            </option>
          ))}
        </select>
      </Field>
      <Field>
        <FieldLabel htmlFor="new-article-body">本文</FieldLabel>
        <textarea
          id="new-article-body"
          value={body}
          onChange={(e) => setBody(e.target.value)}
          rows={10}
          placeholder="本文を書く"
          className="w-full rounded-lg border border-input bg-transparent p-3 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
        />
      </Field>
      {error && <FieldError>{error}</FieldError>}
      <div className="flex gap-2">
        <Button variant="outline" onClick={() => router.push('/articles')} disabled={isLoading}>
          キャンセル
        </Button>
        <Button onClick={handleSubmit} disabled={!canSubmit}>
          {isLoading && <Loader2Icon data-icon="inline-start" className="animate-spin" />}
          下書きとして保存
        </Button>
      </div>
    </FieldGroup>
  )
}
