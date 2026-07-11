// LessonPage 展示单个课时内容，并把学习完成状态上报到 teaching 后端。

import React, { useCallback, useMemo, useState } from 'react'
import { ProgressStatus } from '@chaimir/api-client'
import { Button, Callout } from '@chaimir/ui'
import { CheckCircle, RefreshCw } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const LessonPage: React.FC = () => {
  const { lessonId } = useParams()
  const resource = useAsyncResource(() => api.teaching.getLesson(String(lessonId)), [lessonId])
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const lessonBody = useMemo(() => JSON.stringify(resource.data?.content_ref || {}, null, 2), [resource.data])

  /**
   * handleComplete 上报课时完成进度，服务端保存为权威进度。
   */
  const handleComplete = useCallback(async () => {
    if (!lessonId) {
      return
    }
    setSubmitting(true)
    setError(null)
    setMessage(null)
    try {
      await api.teaching.reportProgress(lessonId, {
        status: ProgressStatus.DONE,
        video_pos: 0,
        duration_sec: 0,
      })
      setMessage('学习进度已同步。')
    } catch (progressError) {
      setError(userFacingErrorMessage(progressError, '进度同步失败，请稍后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [lessonId])

  if (!lessonId) {
    return <EmptyState title="缺少课时信息" description="当前链接没有课时编号。" />
  }
  if (resource.status === 'loading') {
    return <LoadingState title="正在获取课时内容" />
  }
  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }
  if (!resource.data) {
    return <EmptyState title="暂无课时内容" description="当前课时尚未发布内容。" />
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>{resource.data.title}</h1>
          <p className={styles.subtitle}>课时内容和附件由课程资源统一提供。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="已同步">{message}</Callout>}

      <div className={styles.learningGrid}>
        <section className={styles.contentPanel}>
          <h2>课时内容</h2>
          <div className={styles.contentBox}>{lessonBody}</div>
          <Button icon={<CheckCircle size={16} />} loading={submitting} onClick={handleComplete}>
            标记完成
          </Button>
        </section>
        <section className={styles.panel}>
          <h2>学习说明</h2>
          <p className={styles.muted}>视频、文档、实验或仿真入口会随课时内容一起展示。</p>
          <span className={styles.status}>内容类型 {resource.data.content_type}</span>
        </section>
      </div>
    </div>
  )
}

export default LessonPage
