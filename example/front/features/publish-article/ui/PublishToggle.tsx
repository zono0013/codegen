'use client'
import { EyeIcon, EyeOffIcon, Loader2Icon } from 'lucide-react'
import { Badge } from '@/shared/ui/badge'
import { Button } from '@/shared/ui/button'
import { usePublishArticle } from '@/features/publish-article/model/usePublishArticle'

interface PublishToggleProps {
  id: string
  published: boolean
  onSuccess?: () => void
}

export function PublishToggle({ id, published, onSuccess }: PublishToggleProps) {
  const { setPublished, isLoading } = usePublishArticle()

  const handleClick = async () => {
    const updated = await setPublished({ id, published: !published })
    if (updated) onSuccess?.()
  }

  return (
    <div className="flex items-center gap-3">
      <Badge variant={published ? 'default' : 'outline'}>
        {published ? '公開中' : '下書き'}
      </Badge>
      <Button
        variant={published ? 'outline' : 'default'}
        size="sm"
        onClick={handleClick}
        disabled={isLoading}
      >
        {isLoading ? (
          <Loader2Icon data-icon="inline-start" className="animate-spin" />
        ) : published ? (
          <EyeOffIcon data-icon="inline-start" />
        ) : (
          <EyeIcon data-icon="inline-start" />
        )}
        {published ? '下書きに戻す' : '公開する'}
      </Button>
    </div>
  )
}
