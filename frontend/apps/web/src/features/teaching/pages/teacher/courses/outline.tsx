// TeacherCourseOutlinePage 维护课程章节和课时，所有写入调用 teaching 后端接口。

import React, { useCallback, useMemo, useState } from 'react'
import { LessonContentType } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Textarea } from '@chaimir/ui'
import { List, Pencil, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { lessonContentTypeOptions, parseJsonObject } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'


const TeacherCourseOutlinePage: React.FC = () => {
  const { id } = useParams()
  const resource = useAsyncResource(async () => {
    const outline = await api.teaching.getCourseOutline(String(id))
    const lessons = (await Promise.all(outline.chapters.map((chapter) => api.teaching.listLessons(String(chapter.id))))).flat()
    return { ...outline, lessons }
  }, [id])
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
  const [editingChapterId, setEditingChapterId] = useState('')
  const [editingLessonId, setEditingLessonId] = useState('')

  const chapterOptions = useMemo(() => [
    { value: '', label: '选择章节' },
    ...(resource.data?.chapters || []).map((chapter) => ({ value: String(chapter.id), label: chapter.title })),
  ], [resource.data])

  /**
   * saveChapter 创建或更新章节。
   */
  const saveChapter = useCallback(async () => {
    if (!id) return
    setSubmitting('chapter')
    setError(null)
    setMessage(null)
    try {
      const payload = { title: chapterTitle.trim(), sort: Number(chapterSort) }
      if (editingChapterId) await api.teaching.updateChapter(id, editingChapterId, payload)
      else await api.teaching.createChapter(id, payload)
      setMessage(editingChapterId ? '章节已更新。' : '章节已创建。')
      setEditingChapterId('')
      resource.reload()
    } catch (createError) {
      setError(userFacingErrorMessage(createError, '章节保存失败。'))
    } finally {
      setSubmitting(null)
    }
  }, [chapterSort, chapterTitle, editingChapterId, id, resource])

  /**
   * saveLesson 创建或更新课时并绑定内容引用。
   */
  const saveLesson = useCallback(async () => {
    setSubmitting('lesson')
    setError(null)
    setMessage(null)
    try {
      const payload = {
        title: lessonTitle.trim(),
        content_type: Number(lessonType) as LessonContentType,
        content_ref: parseJsonObject(contentRef),
        sort: Number(lessonSort),
      }
      if (editingLessonId) {
        await api.teaching.updateLesson(lessonChapterId, editingLessonId, payload)
        await api.teaching.setLessonContent(editingLessonId, { content_type: payload.content_type, content_ref: payload.content_ref })
      } else {
        await api.teaching.createLesson(lessonChapterId, payload)
      }
      setMessage(editingLessonId ? '课时已更新。' : '课时已创建。')
      setEditingLessonId('')
      resource.reload()
    } catch (createError) {
      setError(userFacingErrorMessage(createError, '课时保存失败。'))
    } finally {
      setSubmitting(null)
    }
  }, [contentRef, editingLessonId, lessonChapterId, lessonSort, lessonTitle, lessonType, resource])

  /** deleteChapter 删除后端确认未被课时依赖的章节。 */
  const deleteChapter = async (chapterId: number) => {
    if (!id || !window.confirm('确定删除这个章节吗？')) return
    try {
      await api.teaching.deleteChapter(id, String(chapterId))
      setMessage('章节已删除。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '章节删除失败，请先处理章节内课时。'))
    }
  }

  /** deleteLesson 删除指定课时并刷新课程大纲。 */
  const deleteLesson = async (chapterId: number, lessonId: number) => {
    if (!window.confirm('确定删除这个课时吗？')) return
    try {
      await api.teaching.deleteLesson(String(chapterId), String(lessonId))
      setMessage('课时已删除。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '课时删除失败，请稍后重试。'))
    }
  }

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
                <div className={styles.chapterHeader}>{chapter.title}<div className={styles.actions}><Button variant="ghost" size="sm" icon={<Pencil size={13} />} onClick={() => { setEditingChapterId(String(chapter.id)); setChapterTitle(chapter.title); setChapterSort(String(chapter.sort)) }}>编辑</Button><Button variant="ghost" size="sm" icon={<Trash2 size={13} />} onClick={() => void deleteChapter(chapter.id)}>删除</Button></div></div>
                {(outline?.lessons || []).filter((lesson) => lesson.chapter_id === chapter.id).map((lesson) => (
                  <div className={styles.lessonRow} key={lesson.id}>
                    <span>{lesson.title}</span>
                    <span className={styles.muted}>排序 {lesson.sort}</span>
                    <div className={styles.actions}><Button variant="ghost" size="sm" icon={<Pencil size={13} />} onClick={() => { setEditingLessonId(String(lesson.id)); setLessonChapterId(String(chapter.id)); setLessonTitle(lesson.title); setLessonType(String(lesson.content_type)); setLessonSort(String(lesson.sort)); setContentRef(JSON.stringify(lesson.content_ref, null, 2)) }}>编辑</Button><Button variant="ghost" size="sm" icon={<Trash2 size={13} />} onClick={() => void deleteLesson(chapter.id, lesson.id)}>删除</Button></div>
                  </div>
                ))}
              </section>
            ))}
          </div>
        </section>

        <section className={styles.panel}>
          <h2>{editingChapterId ? '编辑章节' : '新建章节'}</h2>
          <label className={styles.field}>章节标题<Input fullWidth value={chapterTitle} onChange={(event) => setChapterTitle(event.target.value)} /></label>
          <label className={styles.field}>排序<Input fullWidth value={chapterSort} onChange={(event) => setChapterSort(event.target.value)} /></label>
          <Button loading={submitting === 'chapter'} icon={<Plus size={16} />} onClick={saveChapter}>{editingChapterId ? '保存章节' : '创建章节'}</Button>
        </section>

        <section className={styles.panel}>
          <h2>{editingLessonId ? '编辑课时' : '新建课时'}</h2>
          <label className={styles.field}>所属章节<Select fullWidth value={lessonChapterId} options={chapterOptions} onChange={setLessonChapterId} /></label>
          <label className={styles.field}>课时标题<Input fullWidth value={lessonTitle} onChange={(event) => setLessonTitle(event.target.value)} /></label>
          <label className={styles.field}>内容类型<Select fullWidth value={lessonType} options={lessonContentTypeOptions} onChange={setLessonType} /></label>
          <label className={styles.field}>排序<Input fullWidth value={lessonSort} onChange={(event) => setLessonSort(event.target.value)} /></label>
          <label className={styles.fieldFull}>内容引用<Textarea value={contentRef} onChange={(event) => setContentRef(event.target.value)} /></label>
          <Button loading={submitting === 'lesson'} icon={<Plus size={16} />} onClick={saveLesson}>{editingLessonId ? '保存课时' : '创建课时'}</Button>
        </section>
      </div>
    </div>
  )
}

export default TeacherCourseOutlinePage
