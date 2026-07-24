// TeacherCourseAssignmentEditPage 创建或更新课程作业，所有字段按 teaching 后端契约提交。

import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { GradingMode, LatePolicy } from '@chaimir/api-client'
import { Button, Callout, Input, Select, ResourceState, FormField } from '@chaimir/ui'
import { Edit3, Save, Send } from 'lucide-react'
import { useSearchParams } from 'react-router-dom'
import { api } from '../../../../../../app/api'
import { useAsyncResource } from '../../../../../../hooks'
import styles from '../../../teaching.module.css'
import { formatDateTimeLocalInput, gradingModeOptions, latePolicyOptions, parseDateTimeLocalInput } from '../../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../../utils/userFacingError'

const TeacherCourseAssignmentEditPage: React.FC = () => {
  const [searchParams] = useSearchParams()
  const courseId = searchParams.get('courseId') || ''
  const assignmentId = searchParams.get('assignmentId') || ''
  const chapters = useAsyncResource(() => (courseId ? api.teaching.listChapters(courseId) : Promise.resolve([])), [courseId])
  const assignment = useAsyncResource(() => (assignmentId ? api.teaching.getAssignment(assignmentId) : Promise.resolve(null)), [assignmentId])
  const [title, setTitle] = useState('')
  const [chapterId, setChapterId] = useState('')
  const [dueAt, setDueAt] = useState('')
  const [maxAttempts, setMaxAttempts] = useState('1')
  const [latePolicy, setLatePolicy] = useState(String(LatePolicy.REJECT))
  const [itemCode, setItemCode] = useState('')
  const [itemVersion, setItemVersion] = useState('v1')
  const [score, setScore] = useState('100')
  const [gradingMode, setGradingMode] = useState(String(GradingMode.MANUAL))
  const [judgerCode, setJudgerCode] = useState('')
  const [publishing, setPublishing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!assignment.data) return
    setTitle(assignment.data.assignment.title)
    setChapterId(String(assignment.data.assignment.chapter_id || ''))
    setDueAt(formatDateTimeLocalInput(assignment.data.assignment.due_at))
    setMaxAttempts(String(assignment.data.assignment.max_attempts))
    setLatePolicy(String(assignment.data.assignment.late_policy))
    const firstItem = assignment.data.items[0]
    if (firstItem) {
      setItemCode(firstItem.item_code)
      setItemVersion(firstItem.item_version)
      setScore(String(firstItem.score))
      setGradingMode(String(firstItem.grading_mode))
      setJudgerCode(firstItem.judger_code || '')
    }
  }, [assignment.data])

  const chapterOptions = useMemo(() => [
    { value: '', label: '选择章节' },
    ...(chapters.data || []).map((chapter) => ({ value: String(chapter.id), label: chapter.title })),
  ], [chapters.data])

  /**
   * saveAssignment 创建或更新作业，并按需发布。
   */
  const saveAssignment = useCallback(async (publish: boolean) => {
    setSaving(!publish)
    setPublishing(publish)
    setError(null)
    setMessage(null)
    try {
      const payload = {
        title,
        chapter_id: chapterId,
        due_at: parseDateTimeLocalInput(dueAt),
        max_attempts: Number(maxAttempts),
        late_policy: Number(latePolicy) as LatePolicy,
        late_penalty: {},
        items: [{
          item_code: itemCode,
          item_version: itemVersion,
          score: Number(score),
          seq: 1,
          grading_mode: Number(gradingMode) as GradingMode,
          judger_code: judgerCode,
        }],
      }
      const saved = assignmentId ? await api.teaching.updateAssignment(assignmentId, payload) : await api.teaching.createAssignment(courseId, payload)
      if (publish) {
        await api.teaching.publishAssignment(String(saved.assignment.id))
      }
      setMessage(publish ? '作业已保存并发布。' : '作业已保存。')
      assignment.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '作业保存失败，请检查内容后重试。'))
    } finally {
      setSaving(false)
      setPublishing(false)
    }
  }, [assignment, assignmentId, chapterId, courseId, dueAt, gradingMode, itemCode, itemVersion, judgerCode, latePolicy, maxAttempts, score, title])

  const loading = chapters.status === 'loading' || assignment.status === 'loading'
  const firstError = chapters.error || assignment.error

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Edit3 size={28} />编写作业</h1>
          <p className={styles.subtitle}>将内容中心题目版本绑定为作业题项。</p>
        </div>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="保存成功">{message}</Callout>}
      {firstError && <ResourceState status="error" error={firstError} onRetry={() => { chapters.reload(); assignment.reload() }} />}
      {loading && <ResourceState status="loading" title="正在获取作业配置" />}

      <section className={styles.panel}>
        <h2>基础信息</h2>
        <div className={styles.formGrid}>
          <FormField className={styles.field} label="课程编号"><Input fullWidth value={courseId} readOnly /></FormField>
          <FormField className={styles.field} label="作业标题"><Input fullWidth value={title} onChange={(event) => setTitle(event.target.value)} /></FormField>
          <FormField className={styles.field} label="所属章节"><Select fullWidth value={chapterId} options={chapterOptions} onChange={setChapterId} /></FormField>
          <FormField className={styles.field} label="截止时间"><Input fullWidth type="datetime-local" value={dueAt} onChange={(event) => setDueAt(event.target.value)} /></FormField>
          <FormField className={styles.field} label="最多提交次数"><Input fullWidth value={maxAttempts} onChange={(event) => setMaxAttempts(event.target.value)} /></FormField>
          <FormField className={styles.field} label="迟交策略"><Select fullWidth value={latePolicy} options={latePolicyOptions} onChange={setLatePolicy} /></FormField>
        </div>
      </section>

      <section className={styles.panel}>
        <h2>题项绑定</h2>
        <div className={styles.formGrid}>
          <FormField className={styles.field} label="内容编号"><Input fullWidth value={itemCode} onChange={(event) => setItemCode(event.target.value)} /></FormField>
          <FormField className={styles.field} label="内容版本"><Input fullWidth value={itemVersion} onChange={(event) => setItemVersion(event.target.value)} /></FormField>
          <FormField className={styles.field} label="分值"><Input fullWidth value={score} onChange={(event) => setScore(event.target.value)} /></FormField>
          <FormField className={styles.field} label="评分方式"><Select fullWidth value={gradingMode} options={gradingModeOptions} onChange={setGradingMode} /></FormField>
          <FormField className={styles.fieldFull} label="判题器编号"><Input fullWidth value={judgerCode} onChange={(event) => setJudgerCode(event.target.value)} /></FormField>
        </div>
        <div className={styles.actions}>
          <Button variant="outline" icon={<Save size={16} />} loading={saving} onClick={() => saveAssignment(false)}>保存作业</Button>
          <Button icon={<Send size={16} />} loading={publishing} onClick={() => saveAssignment(true)}>保存并发布</Button>
        </div>
      </section>
    </div>
  )
}

export default TeacherCourseAssignmentEditPage
