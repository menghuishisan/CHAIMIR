// TeacherCourseOutlinePage 维护课程章节和课时，所有写入调用 teaching 后端接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError } from '@chaimir/api-client'
import { LessonContentType } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Textarea } from '@chaimir/ui'
import { List, Plus, RefreshCw } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { lessonContentTypeOptions, parseJsonObject } from '../../../../../utils/index'


const TeacherCourseOutlinePage: React.FC = () => {
  const { id } = useParams()
  const resource = useAsyncResource(() => api.teaching.getCourseOutline(String(id)), [id])
  const [chapterTitle, setChapterTitle] = useState('')
  const [chapterSort, setChapterSort] = useState('1')
  const [lessonChapterId, setLessonChapterId] = useState('')
  const [lessonTitle, setLessonTitle] = useState('')
  const [lessonType, setLessonType] = useState(String(LessonContentType.MARKDOWN))
  const [lessonSort, setLessonSort] = useState('1')
  const [contentRef, setContentRef] = useState('{}')
  const [submitting, setSubmitting] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const chapterOptions = useMemo(() => [
    { value: '', label: '选择章节' },
    ...(resource.data?.chapters || []).map((chapter) => ({ value: String(chapter.id), label: chapter.title })),
  ], [resource.data])

  /**
   * createChapter 创建章节。
   */
  const createChapter = useCallback(async () => {
    if (!id) return
    setSubmitting('chapter')
    setError(null)
    setMessage(null)
    try {
      await api.teaching.createChapter(id, { title: chapterTitle.trim(), sort: Number(chapterSort) })
      setMessage('章节已创建。')
      resource.reload()
    } catch (createError) {
      setError((createError as ApiError).message || '章节创建失败。')
    } finally {
      setSubmitting(null)
    }
  }, [chapterSort, chapterTitle, id, resource])

  /**
   * createLesson 创建课时并绑定内容引用。
   */
  const createLesson = useCallback(async () => {
    setSubmitting('lesson')
    setError(null)
    setMessage(null)
    try {
      await api.teaching.createLesson(lessonChapterId, {
        title: lessonTitle.trim(),
        content_type: Number(lessonType) as LessonContentType,
        content_ref: parseJsonObject(contentRef),
        sort: Number(lessonSort),
      })
      setMessage('课时已创建。')
      resource.reload()
    } catch (createError) {
      setError((createError as ApiError).message || (createError as Error).message || '课时创建失败。')
    } finally {
      setSubmitting(null)
    }
  }, [contentRef, lessonChapterId, lessonSort, lessonTitle, lessonType, resource])

  if (!id) return <EmptyState title="缺少课程编号" description="当前链接没有课程编号。" />
  if (resource.status === 'loading') return <LoadingState title="正在获取课程大纲" />
  if (resource.status === 'error') return <ErrorState error={resource.error} onRetry={resource.reload} />

  const outline = resource.data

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <List size={28} />
            课程大纲管理
          </h1>
          <p className={styles.subtitle}>{outline?.course.name || '课程'} 的章节与课时。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="保存成功">{message}</Callout>}

      <div className={styles.grid}>
        <section className={styles.panel}>
          <h2>章节结构</h2>
          <div className={styles.outline}>
            {(outline?.chapters || []).map((chapter) => (
              <section className={styles.chapter} key={chapter.id}>
                <div className={styles.chapterHeader}>{chapter.title}</div>
                {(outline?.lessons || []).filter((lesson) => lesson.chapter_id === chapter.id).map((lesson) => (
                  <div className={styles.lessonRow} key={lesson.id}>
                    <span>{lesson.title}</span>
                    <span className={styles.muted}>排序 {lesson.sort}</span>
                  </div>
                ))}
              </section>
            ))}
          </div>
        </section>

        <section className={styles.panel}>
          <h2>新建章节</h2>
          <label className={styles.field}>章节标题<Input fullWidth value={chapterTitle} onChange={(event) => setChapterTitle(event.target.value)} /></label>
          <label className={styles.field}>排序<Input fullWidth value={chapterSort} onChange={(event) => setChapterSort(event.target.value)} /></label>
          <Button loading={submitting === 'chapter'} icon={<Plus size={16} />} onClick={createChapter}>创建章节</Button>
        </section>

        <section className={styles.panel}>
          <h2>新建课时</h2>
          <label className={styles.field}>所属章节<Select fullWidth value={lessonChapterId} options={chapterOptions} onChange={setLessonChapterId} /></label>
          <label className={styles.field}>课时标题<Input fullWidth value={lessonTitle} onChange={(event) => setLessonTitle(event.target.value)} /></label>
          <label className={styles.field}>内容类型<Select fullWidth value={lessonType} options={lessonContentTypeOptions} onChange={setLessonType} /></label>
          <label className={styles.field}>排序<Input fullWidth value={lessonSort} onChange={(event) => setLessonSort(event.target.value)} /></label>
          <label className={styles.fieldFull}>内容引用<Textarea value={contentRef} onChange={(event) => setContentRef(event.target.value)} /></label>
          <Button loading={submitting === 'lesson'} icon={<Plus size={16} />} onClick={createLesson}>创建课时</Button>
        </section>
      </div>
    </div>
  )
}

export default TeacherCourseOutlinePage
