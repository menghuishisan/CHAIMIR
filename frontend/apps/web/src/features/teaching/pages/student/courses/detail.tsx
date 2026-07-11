// CourseDetailPage 展示课程章节课时，数据来自 teaching 课程大纲接口。

import React, { useCallback, useMemo, useState } from 'react'
import { ProgressStatus } from '@chaimir/api-client'
import { Button, Callout, FormField, Select, Textarea } from '@chaimir/ui'
import { CheckCircle, FileText, Info, Play, Star } from 'lucide-react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const ratingOptions = [1, 2, 3, 4, 5].map((rating) => ({ value: String(rating), label: `${rating} 分` }))

const CourseDetailPage: React.FC = () => {
  const navigate = useNavigate()
  const { id } = useParams()
  const [rating, setRating] = useState('5')
  const [comment, setComment] = useState('')
  const [reviewing, setReviewing] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(async () => {
    const courseId = String(id || '')
    if (!courseId) throw new Error('缺少课程编号。')
    const [outline, progress] = await Promise.all([
      api.teaching.getCourseOutline(courseId),
      api.teaching.getMyProgress(courseId),
    ])
    return { outline, progress }
  }, [id])
  const outline = resource.data?.outline

  const progressMap = useMemo(() => new Map((resource.data?.progress || []).map((item) => [item.lesson_id, item.status])), [resource.data?.progress])
  const completedLessons = useMemo(
    () => Array.from(progressMap.values()).filter((status) => status === ProgressStatus.DONE).length,
    [progressMap],
  )

  /** handleReview 提交当前学生对课程的评价。 */
  const handleReview = useCallback(async () => {
    if (!id || !comment.trim()) {
      setError('请填写课程评价内容。')
      return
    }
    setReviewing(true)
    setMessage(null)
    setError(null)
    try {
      await api.teaching.reviewCourse(String(id), { rating: Number(rating), comment: comment.trim() })
      setComment('')
      setMessage('课程评价已提交。')
    } catch (reviewError) {
      setError(userFacingErrorMessage(reviewError, '课程评价提交失败，请稍后重试。'))
    } finally {
      setReviewing(false)
    }
  }, [comment, id, rating])

  if (!id) {
    return <EmptyState title="缺少课程信息" description="当前链接没有课程编号。" />
  }
  if (resource.status === 'loading') {
    return <LoadingState title="正在获取课程详情" />
  }
  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }
  if (!outline) {
    return <EmptyState title="暂无课程详情" description="当前课程暂未开放章节课时。" />
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Info size={28} />
            {outline.course.name}
          </h1>
          <p className={styles.subtitle}>{outline.course.description || '暂无课程简介'}</p>
        </div>
        <span className={styles.muted}>已完成 {completedLessons}/{outline.lessons.length} 个课时</span>
      </div>

      <section className={styles.panel}>
        <h2><Star size={18} /> 课程评价</h2>
        {error && <div className={styles.error} role="alert">{error}</div>}
        {message && <Callout variant="success" title="提交成功">{message}</Callout>}
        <div className={styles.toolbar}>
          <FormField label="课程评分" htmlFor="course-rating" required>
            <Select id="course-rating" value={rating} options={ratingOptions} onChange={setRating} />
          </FormField>
          <FormField label="评价内容" htmlFor="course-review" required>
            <Textarea id="course-review" value={comment} rows={3} onChange={(event) => setComment(event.target.value)} />
          </FormField>
        </div>
        <Button icon={<Star size={16} />} loading={reviewing} onClick={handleReview}>提交评价</Button>
      </section>

      <div className={styles.outline}>
        {outline.chapters.map((chapter) => (
          <section className={styles.chapter} key={chapter.id}>
            <div className={styles.chapterHeader}>{chapter.title}</div>
            {outline.lessons.filter((lesson) => lesson.chapter_id === chapter.id).map((lesson) => {
              const done = progressMap.get(lesson.id) === ProgressStatus.DONE
              return (
                <button className={styles.lessonRow} key={lesson.id} type="button" onClick={() => navigate(`/student/courses/${outline.course.id}/lesson/${lesson.id}`)}>
                  {done ? <CheckCircle size={18} /> : <Play size={18} />}
                  <span>{lesson.title}</span>
                  <span className={styles.muted}>{done ? '已完成' : '继续学习'}</span>
                </button>
              )
            })}
          </section>
        ))}
        {outline.chapters.length === 0 && (
          <section className={styles.panel}>
            <FileText size={28} />
            <h2>暂无章节</h2>
            <p className={styles.muted}>教师尚未发布课程章节。</p>
          </section>
        )}
      </div>
    </div>
  )
}

export default CourseDetailPage
