// TeacherCourseEditPage 创建或更新课程基础信息，复用 teaching 后端课程接口。

import React, { useCallback, useEffect, useState } from 'react'
import { CourseType, TeachingDifficulty } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Textarea, ResourceState, FormField } from '@chaimir/ui'
import { Edit, Save, Send } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { courseTypeOptions, formatDateTimeLocalInput, parseDateTimeLocalInput, teachingDifficultyOptions } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const TeacherCourseEditPage: React.FC = () => {
  const [searchParams] = useSearchParams()
  const courseId = searchParams.get('id') || ''
  const course = useAsyncResource(
    () => courseId ? api.teaching.getCourseOutline(courseId).then((outline) => outline.course) : Promise.resolve(null),
    [courseId],
    () => false,
  )
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [type, setType] = useState(String(CourseType.MIXED))
  const [difficulty, setDifficulty] = useState(String(TeachingDifficulty.INTRO))
  const [semester, setSemester] = useState('')
  const [credits, setCredits] = useState('2')
  const [startAt, setStartAt] = useState('')
  const [endAt, setEndAt] = useState('')
  const [saving, setSaving] = useState(false)
  const [publishing, setPublishing] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const current = course.data
    if (!current) return
    setName(current.name)
    setDescription(current.description)
    setType(String(current.type))
    setDifficulty(String(current.difficulty))
    setSemester(current.semester)
    setCredits(String(current.credits))
    setStartAt(formatDateTimeLocalInput(current.start_at))
    setEndAt(formatDateTimeLocalInput(current.end_at))
  }, [course.data])

  /**
   * saveCourse 创建或更新课程，并可立即发布。
   */
  const saveCourse = useCallback(async (publish: boolean) => {
    setSaving(!publish)
    setPublishing(publish)
    setError(null)
    setMessage(null)
    try {
      const payload = {
        name,
        description,
        type: Number(type) as CourseType,
        difficulty: Number(difficulty) as TeachingDifficulty,
        semester,
        credits: Number(credits),
        schedule: {},
        start_at: parseDateTimeLocalInput(startAt),
        end_at: parseDateTimeLocalInput(endAt),
      }
      const saved = courseId ? await api.teaching.updateCourse(courseId, payload) : await api.teaching.createCourse(payload)
      if (publish) {
        await api.teaching.publishCourse(String(saved.id))
      }
      setMessage(publish ? '课程已保存并发布。' : '课程已保存。')
      if (courseId) course.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '课程保存失败，请检查内容后重试。'))
    } finally {
      setSaving(false)
      setPublishing(false)
    }
  }, [course, courseId, credits, description, difficulty, endAt, name, semester, startAt, type])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Edit size={28} />编辑基础信息</h1>
          <p className={styles.subtitle}>保存后可继续完善课程，发布后学生即可查看。</p>
        </div>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="保存成功">{message}</Callout>}
      {courseId && course.status === 'error' && <ResourceState status="error" error={course.error} onRetry={course.reload} />}
      {courseId && course.status === 'loading' && <ResourceState status="loading" title="正在读取课程" />}

      <section className={styles.panel}>
        <div className={styles.formGrid}>
          <FormField className={styles.field} label="课程名称"><Input fullWidth value={name} onChange={(event) => setName(event.target.value)} /></FormField>
          <FormField className={styles.field} label="学期"><Input fullWidth value={semester} onChange={(event) => setSemester(event.target.value)} /></FormField>
          <FormField className={styles.field} label="课程类型"><Select fullWidth value={type} options={courseTypeOptions} onChange={setType} /></FormField>
          <FormField className={styles.field} label="难度"><Select fullWidth value={difficulty} options={teachingDifficultyOptions} onChange={setDifficulty} /></FormField>
          <FormField className={styles.field} label="学分"><Input fullWidth value={credits} onChange={(event) => setCredits(event.target.value)} /></FormField>
          <FormField className={styles.field} label="开课时间"><Input fullWidth type="datetime-local" value={startAt} onChange={(event) => setStartAt(event.target.value)} /></FormField>
          <FormField className={styles.field} label="结课时间"><Input fullWidth type="datetime-local" value={endAt} onChange={(event) => setEndAt(event.target.value)} /></FormField>
          <FormField className={styles.fieldFull} label="课程简介"><Textarea value={description} onChange={(event) => setDescription(event.target.value)} rows={5} /></FormField>
        </div>
        <div className={styles.actions}>
          <Button variant="outline" icon={<Save size={16} />} loading={saving} onClick={() => saveCourse(false)}>保存课程</Button>
          <Button icon={<Send size={16} />} loading={publishing} onClick={() => saveCourse(true)}>保存并发布</Button>
        </div>
      </section>
    </div>
  )
}

export default TeacherCourseEditPage
