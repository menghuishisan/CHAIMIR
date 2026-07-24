// LessonPage 展示单个课时内容，并把学习完成状态上报到 teaching 后端。

import React, { useCallback, useState } from 'react'
import type { LessonContentRef, LessonExperimentRef, LessonMarkdownRef, LessonSimulationRef, LessonVideoRef, LessonAttachmentRef } from '@chaimir/api-client'
import { LessonContentType, ProgressStatus } from '@chaimir/api-client'
import { Button, Callout, DescriptionList, ResourceState } from '@chaimir/ui'
import { CheckCircle, FileText, FlaskConical, PlaySquare, RefreshCw } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { lessonContentTypeLabel } from '../../../../../utils'

const LessonPage: React.FC = () => {
  const { lessonId } = useParams()
  const resource = useAsyncResource(() => api.teaching.getLesson(String(lessonId)), [lessonId])
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

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
    return <ResourceState status="empty" title="缺少课时信息" description="当前链接没有课时编号。" />
  }
  if (resource.status === 'loading') {
    return <ResourceState status="loading" title="正在获取课时内容" />
  }
  if (resource.status === 'error') {
    return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
  }
  if (!resource.data) {
    return <ResourceState status="empty" title="暂无课时内容" description="当前课时尚未发布内容。" />
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
          <LessonContent type={resource.data.content_type} value={resource.data.content_ref} />
          <Button icon={<CheckCircle size={16} />} loading={submitting} onClick={handleComplete}>
            标记完成
          </Button>
        </section>
        <section className={styles.panel}>
          <h2>学习说明</h2>
          <p className={styles.muted}>视频、文档、实验或仿真入口会随课时内容一起展示。</p>
          <span className={styles.status}>{lessonContentTypeLabel(resource.data.content_type)}</span>
        </section>
      </div>
    </div>
  )
}

export default LessonPage

/** LessonContent 按课时类型渲染可读正文或资源摘要，不暴露底层对象。 */
function LessonContent({ type, value }: { type: LessonContentType; value: LessonContentRef }): React.ReactElement {
  if (type === LessonContentType.MARKDOWN) {
    return <div className={styles.contentBox}>{(value as LessonMarkdownRef).markdown}</div>
  }
  if (type === LessonContentType.VIDEO) {
    const ref = value as LessonVideoRef
    return <div className={styles.resourceSummary}><PlaySquare size={22} /><DescriptionList columns={2} items={[{ key: 'name', label: '视频', value: ref.file_name }, { key: 'duration', label: '时长', value: formatDuration(ref.duration_sec) }]} /></div>
  }
  if (type === LessonContentType.ATTACHMENT) {
    const ref = value as LessonAttachmentRef
    return <div className={styles.resourceSummary}><FileText size={22} /><DescriptionList items={[{ key: 'name', label: '附件', value: ref.file_name }]} /></div>
  }
  if (type === LessonContentType.EXPERIMENT) {
    const ref = value as LessonExperimentRef
    return <div className={styles.resourceSummary}><FlaskConical size={22} /><DescriptionList items={[{ key: 'experiment', label: '实验', value: ref.experiment_id }]} /></div>
  }
  const ref = value as LessonSimulationRef
  return <div className={styles.resourceSummary}><PlaySquare size={22} /><DescriptionList columns={2} items={[{ key: 'code', label: '仿真场景', value: ref.package_code }, { key: 'version', label: '版本', value: ref.version }]} /></div>
}

/** formatDuration 把秒数转换为便于阅读的分钟和秒。 */
function formatDuration(seconds: number): string {
  const safeSeconds = Math.max(0, Math.round(seconds))
  return `${Math.floor(safeSeconds / 60)} 分 ${safeSeconds % 60} 秒`
}
