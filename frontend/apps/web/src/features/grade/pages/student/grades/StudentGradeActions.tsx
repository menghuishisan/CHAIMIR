// StudentGradeActions 提供学生成绩申诉、成绩单生成和下载授权操作。

import React, { useCallback, useState } from 'react'
import type { GradeTranscript } from '@chaimir/api-client'
import { TranscriptScope } from '@chaimir/api-client'
import { Button, Callout, FormField, Input, Textarea } from '@chaimir/ui'
import { Download, FileText, MessageSquareWarning } from 'lucide-react'
import { api } from '../../../../../app/api'
import { formatDateTime } from '../../../../../utils'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import styles from '../../grade.module.css'

export interface StudentGradeActionsProps {
  studentId: string
  semesterId: string
}

/** StudentGradeActions 只使用后端持久化记录，不在本地伪造成绩单或申诉状态。 */
export function StudentGradeActions({ studentId, semesterId }: StudentGradeActionsProps): React.ReactElement {
  const [courseId, setCourseId] = useState('')
  const [reason, setReason] = useState('')
  const [transcript, setTranscript] = useState<GradeTranscript | null>(null)
  const [pending, setPending] = useState<'appeal' | 'generate' | 'download' | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  /** runAction 统一操作反馈并保留用户输入用于失败重试。 */
  const runAction = useCallback(async (kind: Exclude<typeof pending, null>, action: () => Promise<string>) => {
    setPending(kind)
    setMessage(null)
    setError(null)
    try {
      setMessage(await action())
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '操作未完成，请稍后重试。'))
    } finally {
      setPending(null)
    }
  }, [])

  /** submitAppeal 提交指定课程的成绩申诉。 */
  const submitAppeal = useCallback(() => {
    if (!courseId.trim() || !reason.trim()) {
      setError('请填写课程编号和申诉原因。')
      return
    }
    void runAction('appeal', async () => {
      await api.grade.submitAppeal({ course_id: courseId.trim(), reason: reason.trim() })
      setCourseId('')
      setReason('')
      return '成绩申诉已提交。'
    })
  }, [courseId, reason, runAction])

  /** generateTranscript 创建服务端成绩单记录。 */
  const generateTranscript = useCallback(() => {
    void runAction('generate', async () => {
      const generated = await api.grade.generateTranscript({
        student_id: studentId,
        scope: semesterId ? TranscriptScope.SEMESTER : TranscriptScope.FULL,
        semester_id: semesterId || undefined,
      })
      setTranscript(generated)
      return '成绩单已生成，可以申请下载授权。'
    })
  }, [runAction, semesterId, studentId])

  /** authorizeTranscriptDownload 请求后端成绩单下载授权。 */
  const authorizeTranscriptDownload = useCallback(() => {
    if (!transcript) return
    void runAction('download', async () => {
      const grant = await api.grade.downloadTranscript(transcript.id)
      return `下载授权已签发，有效期至 ${formatDateTime(grant.expires_at)}。`
    })
  }, [runAction, transcript])

  return (
    <div className={styles.split}>
      <section className={styles.panel}>
        <h2><MessageSquareWarning size={18} /> 成绩申诉</h2>
        <FormField label="课程编号" htmlFor="appeal-course-id" required>
          <Input id="appeal-course-id" fullWidth value={courseId} onChange={(event) => setCourseId(event.target.value)} />
        </FormField>
        <FormField label="申诉原因" htmlFor="appeal-reason" required>
          <Textarea id="appeal-reason" fullWidth rows={4} value={reason} onChange={(event) => setReason(event.target.value)} />
        </FormField>
        <Button icon={<MessageSquareWarning size={16} />} loading={pending === 'appeal'} onClick={submitAppeal}>提交申诉</Button>
      </section>

      <section className={styles.panel}>
        <h2><FileText size={18} /> 成绩单</h2>
        <p className={styles.muted}>{semesterId ? '生成当前学期成绩单。' : '生成完整成绩单。'}</p>
        <div className={styles.actions}>
          <Button icon={<FileText size={16} />} loading={pending === 'generate'} onClick={generateTranscript}>生成成绩单</Button>
          <Button variant="outline" icon={<Download size={16} />} loading={pending === 'download'} disabled={!transcript} onClick={authorizeTranscriptDownload}>获取下载授权</Button>
        </div>
        {transcript && <p className={styles.muted}>最近生成：{formatDateTime(transcript.generated_at)}</p>}
      </section>

      {error && <div className={`${styles.error} ${styles.wide}`} role="alert">{error}</div>}
      {message && <Callout className={styles.wide} variant="success" title="操作完成">{message}</Callout>}
    </div>
  )
}
