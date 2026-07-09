// TeacherGradingPage 按作业编号查询提交记录，并调用 teaching 后端批改接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError, Submission } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Table, Textarea } from '@chaimir/ui'
import { Check, CheckSquare, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../teaching.module.css'
import { formatDateTime } from '../../../../../utils/index'

const TeacherGradingPage: React.FC = () => {
  const [assignmentId, setAssignmentId] = useState('')
  const [submissionId, setSubmissionId] = useState('')
  const [score, setScore] = useState('')
  const [comment, setComment] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(() => (assignmentId ? api.teaching.getSubmissions(assignmentId, { page: 1, size: 20 }) : Promise.resolve({ list: [], total: 0, page: 1, size: 20 })), [assignmentId])

  /**
   * gradeSubmission 提交人工批改分数和评语。
   */
  const gradeSubmission = useCallback(async () => {
    setError(null)
    setMessage(null)
    try {
      await api.teaching.gradeSubmission(submissionId, { score: Number(score), comment })
      setMessage('批改结果已保存。')
      resource.reload()
    } catch (actionError) {
      setError((actionError as ApiError).message || '批改保存失败，请稍后重试。')
    }
  }, [comment, resource, score, submissionId])

  const columns = useMemo<TableColumn<Submission>[]>(() => [
    { key: 'student', title: '学生编号', dataIndex: 'student_id', priority: 'primary' },
    { key: 'attempt', title: '提交次数', render: (row) => `第 ${row.attempt_no} 次` },
    { key: 'score', title: '最终得分', render: (row) => (row.final_score === undefined ? '待评分' : row.final_score.toFixed(1)) },
    { key: 'late', title: '迟交', render: (row) => (row.is_late ? '是' : '否') },
    { key: 'submitted', title: '提交时间', render: (row) => formatDateTime(row.submitted_at) },
    {
      key: 'actions',
      title: '操作',
      render: (row) => <Button variant="outline" size="sm" icon={<Check size={14} />} onClick={() => setSubmissionId(String(row.id))}>选择批改</Button>,
    },
  ], [])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><CheckSquare size={28} />批改中心</h1>
          <p className={styles.subtitle}>输入作业编号获取提交记录，并保存人工批改结果。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="保存成功">{message}</Callout>}
      <section className={styles.panel}>
        <h2>作业筛选</h2>
        <div className={styles.formGrid}>
          <label className={styles.field}>作业编号<Input fullWidth value={assignmentId} onChange={(event) => setAssignmentId(event.target.value)} /></label>
          <label className={styles.field}>提交编号<Input fullWidth value={submissionId} onChange={(event) => setSubmissionId(event.target.value)} /></label>
          <label className={styles.field}>分数<Input fullWidth value={score} onChange={(event) => setScore(event.target.value)} /></label>
          <label className={styles.fieldFull}>评语<Textarea value={comment} onChange={(event) => setComment(event.target.value)} /></label>
        </div>
        <Button icon={<Check size={16} />} onClick={gradeSubmission}>保存批改结果</Button>
      </section>
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取提交记录" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无提交记录" emptyDescription="输入作业编号后查看学生提交。" ariaLabel="教师批改提交列表" />
        </div>
      )}
    </div>
  )
}

export default TeacherGradingPage
