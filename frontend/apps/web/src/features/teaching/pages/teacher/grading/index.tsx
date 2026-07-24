// TeacherGradingPage 按作业编号查询提交记录，并调用 teaching 后端批改接口。

import React, { useCallback, useMemo, useState } from 'react'
import { AssignmentStatus, type Submission } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Table, Textarea, ResourceState, FormField } from '@chaimir/ui'
import { Check, CheckSquare, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource, usePendingAction } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { formatDateTime } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { CourseGradebookPanel } from './CourseGradebookPanel'

const TeacherGradingPage: React.FC = () => {
  const [assignmentId, setAssignmentId] = useState('')
  const [courseId, setCourseId] = useState('')
  const [score, setScore] = useState('')
  const [comment, setComment] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [selectedSubmission, setSelectedSubmission] = useState<Submission>()
  const { pendingAction, runPendingAction } = usePendingAction()
  const courses = useAsyncResource(() => api.teaching.getCourses({ role: 'teacher', page: 1, size: 100 }), [])
  const assignments = useAsyncResource(() => (courseId ? api.teaching.listAssignments(courseId) : Promise.resolve([])), [courseId])
  const resource = useAsyncResource(() => (assignmentId ? api.teaching.getSubmissions(assignmentId, { page: 1, size: 20 }) : Promise.resolve({ list: [], total: 0, page: 1, size: 20 })), [assignmentId])

  /**
   * gradeSubmission 提交人工批改分数和评语。
   */
  const gradeSubmission = useCallback(async () => {
    setError(null)
    setMessage(null)
    try {
      if (!selectedSubmission) {
        setError('请先选择一条提交记录。')
        return
      }
      const parsedScore = Number(score)
      if (!Number.isFinite(parsedScore) || parsedScore < 0 || parsedScore > 100) {
        setError('请输入 0 到 100 之间的分数。')
        return
      }
      await api.teaching.gradeSubmission(selectedSubmission.id, { score: parsedScore, comment: comment.trim() })
      setMessage('批改结果已保存。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '批改保存失败，请稍后重试。'))
    }
  }, [comment, resource, score, selectedSubmission])

  /** selectSubmission 读取完整提交内容并载入批改表单。 */
  const selectSubmission = async (id: string) => {
    setError(null)
    try {
      const detail = await api.teaching.getSubmission(id)
      setSelectedSubmission(detail)
      setScore(String(detail.final_score ?? detail.auto_score ?? 0))
      setComment(detail.comment || '')
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '提交详情读取失败，请稍后重试。'))
    }
  }

  const columns = useMemo<TableColumn<Submission>[]>(() => [
    { key: 'student', title: '提交人', render: (row) => row.student_name, priority: 'primary' },
    { key: 'studentNo', title: '学号', render: (row) => row.student_no || '未设置' },
    { key: 'attempt', title: '提交次数', render: (row) => `第 ${row.attempt_no} 次` },
    { key: 'score', title: '最终得分', render: (row) => (row.final_score === undefined ? '待评分' : row.final_score.toFixed(1)) },
    { key: 'late', title: '迟交', render: (row) => (row.is_late ? '是' : '否') },
    { key: 'submitted', title: '提交时间', render: (row) => formatDateTime(row.submitted_at) },
    {
      key: 'actions',
      title: '操作',
      render: (row) => <Button variant="outline" size="sm" icon={<Check size={14} />} loading={pendingAction === `select-${row.id}`} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction(`select-${row.id}`, () => selectSubmission(row.id))}>选择批改</Button>,
    },
  ], [pendingAction, runPendingAction])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><CheckSquare size={28} />批改中心</h1>
          <p className={styles.subtitle}>按课程和作业查看提交记录，并保存人工批改结果。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="保存成功">{message}</Callout>}
      <section className={styles.panel}>
        <h2>作业筛选</h2>
        <div className={styles.formGrid}>
          <FormField className={styles.field} label="课程">
            <Select fullWidth value={courseId} placeholder={courses.status === 'loading' ? '正在读取课程' : '请选择课程'} options={(courses.data?.list || []).map((course) => ({ value: course.id, label: course.name }))} onChange={(value) => { setCourseId(value); setAssignmentId(''); setSelectedSubmission(undefined) }} />
          </FormField>
          <FormField className={styles.field} label="作业">
            <Select fullWidth value={assignmentId} disabled={!courseId || assignments.status === 'loading'} placeholder={!courseId ? '请先选择课程' : assignments.status === 'loading' ? '正在读取作业' : '请选择作业'} options={(assignments.data || []).map((assignment) => ({ value: assignment.id, label: `${assignment.title}${assignment.status === AssignmentStatus.DRAFT ? '（草稿）' : ''}` }))} onChange={(value) => { setAssignmentId(value); setSelectedSubmission(undefined) }} />
          </FormField>
          <FormField className={styles.field} label="分数"><Input fullWidth value={score} onChange={(event) => setScore(event.target.value)} /></FormField>
          <FormField className={styles.fieldFull} label="评语"><Textarea value={comment} onChange={(event) => setComment(event.target.value)} /></FormField>
        </div>
        <Button loading={pendingAction === 'grade'} disabled={Boolean(pendingAction) || !selectedSubmission} icon={<Check size={16} />} onClick={() => void runPendingAction('grade', gradeSubmission)}>保存批改结果</Button>
        {selectedSubmission && <section className={styles.answerPreview}><h3>学生作答</h3><p>{typeof selectedSubmission.content.answer === 'string' && selectedSubmission.content.answer.trim() ? selectedSubmission.content.answer : '学生未填写补充答案。'}</p></section>}
      </section>
      <CourseGradebookPanel courseId={courseId} />
      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取提交记录" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无提交记录" emptyDescription={assignmentId ? '当前作业还没有学生提交。' : '选择课程和作业后查看学生提交。'} ariaLabel="教师批改提交列表" />
        </div>
      )}
    </div>
  )
}

export default TeacherGradingPage
