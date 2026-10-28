// 课程发帖和回复共用的单步发布表单,保留失败输入并关联服务端父帖。

import { useId, useState } from 'react'
import type { FormEvent } from 'react'
import type { TeachingPost } from '@chaimir/api-client'
import { Button, FormField, Textarea, toast } from '@chaimir/ui'
import { Send } from 'lucide-react'
import { api } from '../../../app/api'
import { userFacingErrorMessage } from '../../../utils/userFacingError'

interface CoursePostComposerProps {
  courseId: string
  parent?: TeachingPost
  onPosted: () => void
  onSubmittingChange?: (submitting: boolean) => void
}

/** CoursePostComposer 只在发布成功后清空输入,回复复用同一个创建接口。 */
export function CoursePostComposer({ courseId, parent, onPosted, onSubmittingChange }: CoursePostComposerProps) {
  const inputId = useId()
  const [content, setContent] = useState('')
  const [error, setError] = useState<string>()
  const [submitting, setSubmitting] = useState(false)

  /** submit 校验正文并等待服务端确认,失败时保留本次输入。 */
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (submitting) return
    const text = content.trim()
    if (!text) {
      setError(parent ? '请输入回复内容' : '请输入要发布的内容')
      return
    }
    setError(undefined)
    setSubmitting(true)
    onSubmittingChange?.(true)
    try {
      await api.teaching.createPost(courseId, { content: text, parent_id: parent?.id })
      setContent('')
      toast.success(parent ? '回复已发布' : '讨论已发布')
      onPosted()
    } catch (cause) {
      setError(userFacingErrorMessage(cause, '发布没有成功,请稍后重试。'))
    } finally {
      setSubmitting(false)
      onSubmittingChange?.(false)
    }
  }

  return (
    <form onSubmit={(event) => void submit(event)} noValidate>
      {parent ? (
        <blockquote className="mb-4 whitespace-pre-wrap break-words border-l-2 border-primary pl-3 text-sm text-ink-sub">
          {parent.content}
        </blockquote>
      ) : null}
      <FormField htmlFor={inputId} label={parent ? '回复内容' : '发表讨论'} required error={error}>
        <Textarea
          id={inputId}
          value={content}
          rows={3}
          placeholder={parent ? '写下你的回复' : '说说你的想法或提出问题'}
          invalid={Boolean(error)}
          disabled={submitting}
          onChange={(event) => {
            setContent(event.target.value)
            setError(undefined)
          }}
        />
      </FormField>
      <div className="mt-3 flex justify-end">
        <Button type="submit" leftIcon={Send} loading={submitting}>
          {parent ? '发布回复' : '发布讨论'}
        </Button>
      </div>
    </form>
  )
}
