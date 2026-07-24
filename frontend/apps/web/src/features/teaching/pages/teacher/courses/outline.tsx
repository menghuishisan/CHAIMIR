// TeacherCourseOutlinePage 维护课程章节和课时，所有写入调用 teaching 后端接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { LessonContentRef, LessonExperimentRef, LessonMarkdownRef, LessonSimulationRef, LessonVideoRef, LessonAttachmentRef } from '@chaimir/api-client'
import { LessonContentType } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Textarea, useConfirm, ResourceState, FormField } from '@chaimir/ui'
import { List, Pencil, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { lessonContentTypeOptions } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'


const TeacherCourseOutlinePage: React.FC = () => {
  const confirm = useConfirm()
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
  const [contentRef, setContentRef] = useState<LessonContentRef>(() => createLessonContentRef(LessonContentType.MARKDOWN))
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
        content_ref: contentRef,
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
  const deleteChapter = async (chapterId: string) => {
    if (!id) return
    const confirmed = await confirm({ title: '删除章节', description: '只有不再包含课时的章节才能删除，确定继续吗？', confirmLabel: '确认删除' })
    if (!confirmed) return
    try {
      await api.teaching.deleteChapter(id, chapterId)
      setMessage('章节已删除。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '章节删除失败，请先处理章节内课时。'))
    }
  }

  /** deleteLesson 删除指定课时并刷新课程大纲。 */
  const deleteLesson = async (chapterId: string, lessonId: string) => {
    const confirmed = await confirm({ title: '删除课时', description: '删除后该课时将从课程大纲中移除，确定继续吗？', confirmLabel: '确认删除' })
    if (!confirmed) return
    try {
      await api.teaching.deleteLesson(chapterId, lessonId)
      setMessage('课时已删除。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '课时删除失败，请稍后重试。'))
    }
  }

  if (!id) return <ResourceState status="empty" title="缺少课程编号" description="当前链接没有课程编号。" />
  if (resource.status === 'loading') return <ResourceState status="loading" title="正在获取课程大纲" />
  if (resource.status === 'error') return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />

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
                    <div className={styles.actions}><Button variant="ghost" size="sm" icon={<Pencil size={13} />} onClick={() => { setEditingLessonId(String(lesson.id)); setLessonChapterId(String(chapter.id)); setLessonTitle(lesson.title); setLessonType(String(lesson.content_type)); setLessonSort(String(lesson.sort)); setContentRef(lesson.content_ref) }}>编辑</Button><Button variant="ghost" size="sm" icon={<Trash2 size={13} />} onClick={() => void deleteLesson(chapter.id, lesson.id)}>删除</Button></div>
                  </div>
                ))}
              </section>
            ))}
          </div>
        </section>

        <section className={styles.panel}>
          <h2>{editingChapterId ? '编辑章节' : '新建章节'}</h2>
          <FormField className={styles.field} label="章节标题"><Input fullWidth value={chapterTitle} onChange={(event) => setChapterTitle(event.target.value)} /></FormField>
          <FormField className={styles.field} label="排序"><Input fullWidth value={chapterSort} onChange={(event) => setChapterSort(event.target.value)} /></FormField>
          <Button loading={submitting === 'chapter'} icon={<Plus size={16} />} onClick={saveChapter}>{editingChapterId ? '保存章节' : '创建章节'}</Button>
        </section>

        <section className={styles.panel}>
          <h2>{editingLessonId ? '编辑课时' : '新建课时'}</h2>
          <FormField className={styles.field} label="所属章节"><Select fullWidth value={lessonChapterId} options={chapterOptions} onChange={setLessonChapterId} /></FormField>
          <FormField className={styles.field} label="课时标题"><Input fullWidth value={lessonTitle} onChange={(event) => setLessonTitle(event.target.value)} /></FormField>
          <FormField className={styles.field} label="内容类型"><Select fullWidth value={lessonType} options={lessonContentTypeOptions} onChange={(nextType) => { setLessonType(nextType); setContentRef(createLessonContentRef(Number(nextType) as LessonContentType)) }} /></FormField>
          <FormField className={styles.field} label="排序"><Input fullWidth value={lessonSort} onChange={(event) => setLessonSort(event.target.value)} /></FormField>
          <LessonContentFields type={Number(lessonType) as LessonContentType} value={contentRef} onChange={setContentRef} />
          <Button loading={submitting === 'lesson'} icon={<Plus size={16} />} onClick={saveLesson}>{editingLessonId ? '保存课时' : '创建课时'}</Button>
        </section>
      </div>
    </div>
  )
}

export default TeacherCourseOutlinePage

/** createLessonContentRef 为课时类型生成唯一合法的初始内容结构。 */
function createLessonContentRef(type: LessonContentType): LessonContentRef {
  if (type === LessonContentType.VIDEO) return { object_ref: '', file_name: '', duration_sec: 0 }
  if (type === LessonContentType.MARKDOWN) return { markdown: '' }
  if (type === LessonContentType.ATTACHMENT) return { object_ref: '', file_name: '' }
  if (type === LessonContentType.EXPERIMENT) return { experiment_id: '' }
  return { package_code: '', version: '' }
}

/** LessonContentFields 按课时类型显示用户可理解的资源字段。 */
function LessonContentFields({ type, value, onChange }: { type: LessonContentType; value: LessonContentRef; onChange: (value: LessonContentRef) => void }): React.ReactElement {
  if (type === LessonContentType.MARKDOWN) {
    const ref = value as LessonMarkdownRef
    return <FormField className={styles.fieldFull} label="图文正文"><Textarea rows={8} value={ref.markdown} onChange={(event) => onChange({ markdown: event.target.value })} /></FormField>
  }
  if (type === LessonContentType.VIDEO) {
    const ref = value as LessonVideoRef
    return <><FormField className={styles.field} label="视频名称"><Input fullWidth value={ref.file_name} onChange={(event) => onChange({ ...ref, file_name: event.target.value })} /></FormField><FormField className={styles.field} label="视频资源"><Input fullWidth value={ref.object_ref} onChange={(event) => onChange({ ...ref, object_ref: event.target.value })} /></FormField><FormField className={styles.field} label="时长（秒）"><Input fullWidth type="number" min={0} value={ref.duration_sec} onChange={(event) => onChange({ ...ref, duration_sec: Number(event.target.value) })} /></FormField></>
  }
  if (type === LessonContentType.ATTACHMENT) {
    const ref = value as LessonAttachmentRef
    return <><FormField className={styles.field} label="附件名称"><Input fullWidth value={ref.file_name} onChange={(event) => onChange({ ...ref, file_name: event.target.value })} /></FormField><FormField className={styles.field} label="附件资源"><Input fullWidth value={ref.object_ref} onChange={(event) => onChange({ ...ref, object_ref: event.target.value })} /></FormField></>
  }
  if (type === LessonContentType.EXPERIMENT) {
    const ref = value as LessonExperimentRef
    return <FormField className={styles.fieldFull} label="实验编号"><Input fullWidth value={ref.experiment_id} onChange={(event) => onChange({ experiment_id: event.target.value })} /></FormField>
  }
  const ref = value as LessonSimulationRef
  return <><FormField className={styles.field} label="仿真包编号"><Input fullWidth value={ref.package_code} onChange={(event) => onChange({ ...ref, package_code: event.target.value })} /></FormField><FormField className={styles.field} label="仿真包版本"><Input fullWidth value={ref.version} onChange={(event) => onChange({ ...ref, version: event.target.value })} /></FormField></>
}
