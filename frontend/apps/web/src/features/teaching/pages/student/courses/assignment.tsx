// AssignmentPage 展示学生作业详情，保存服务端草稿并提交作答。

import React, { useCallback, useEffect, useState } from 'react'
import { Button, Callout, Textarea, ResourceState, FormField } from '@chaimir/ui'
import { Save, Send } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource, usePendingAction } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { formatDateTime } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const AssignmentPage: React.FC = () => {
  const navigate = useNavigate()
  const { id } = useParams()
  const [content, setContent] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const { pendingAction, runPendingAction } = usePendingAction()
  const assignmentId = id || ''
  const resource = useAsyncResource(async () => {
    const [detail, draft] = await Promise.all([
      api.teaching.getAssignment(assignmentId),
      api.teaching.getDraft(assignmentId),
    ])
    return { detail, draft }
  }, [assignmentId])

  useEffect(() => {
    const answer = resource.data?.draft.content.answer
    if (typeof answer === 'string') {
      setContent(answer)
    }
  }, [resource.data])

  /**
   * saveDraft 把当前作答保存到服务端草稿，刷新后仍以服务端为准。
   */
  const saveDraft = useCallback(async () => {
    setError(null)
    setMessage(null)
    try {
      await api.teaching.saveDraft(assignmentId, { content: { answer: content } })
      setMessage('草稿已保存。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '草稿保存失败，请稍后重试。'))
    }
  }, [assignmentId, content, resource])

  /**
   * submitAnswer 提交作答后跳转到该作业结果页。
   */
  const submitAnswer = useCallback(async () => {
    setError(null)
    setMessage(null)
    try {
      const submission = await api.teaching.submitAssignment(assignmentId, { content_ref: { answer: content } })
      navigate(`/student/courses/assignment/${assignmentId}/result?submissionId=${encodeURIComponent(submission.id)}`)
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '作业提交失败，请稍后重试。'))
    }
  }, [assignmentId, content, navigate])

  const detail = resource.data?.detail

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>{detail?.assignment.title || '作业作答'}</h1>
          <p className={styles.subtitle}>截止时间：{detail ? formatDateTime(detail.assignment.due_at) : '正在获取'}</p>
        </div>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="保存成功">{message}</Callout>}
      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取作业" />}
      {detail && (
        <section className={styles.panel}>
          <h2>作业要求</h2>
          <div className={styles.outline}>
            {detail.items.map((item) => (
              <div className={styles.card} key={item.id}>
                <strong>{item.title || item.item_code}</strong>
                <span className={styles.muted}>分值 {item.score} · 版本 {item.item_version}</span>
              </div>
            ))}
          </div>
          <FormField className={styles.fieldFull} label="作答内容"><Textarea value={content} onChange={(event) => setContent(event.target.value)} rows={10} /></FormField>
          <div className={styles.actions}>
            <Button variant="outline" icon={<Save size={16} />} loading={pendingAction === 'draft'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('draft', saveDraft)}>保存草稿</Button>
            <Button icon={<Send size={16} />} loading={pendingAction === 'submit'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('submit', submitAnswer)}>提交作答</Button>
          </div>
        </section>
      )}
    </div>
  )
}

export default AssignmentPage
