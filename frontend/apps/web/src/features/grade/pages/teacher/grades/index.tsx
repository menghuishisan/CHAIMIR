// TeacherGradesPage 展示教师成绩报送记录，并通过后端提交新的成绩审核。

import React, { useCallback, useMemo, useState } from 'react'
import type { GradeReview } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Table, Textarea } from '@chaimir/ui'
import { Calculator, HelpCircle, RefreshCw, Send } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../grade.module.css'
import { formatDateTime, gradeReviewStatusLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'


const TeacherGradesPage: React.FC = () => {
  const navigate = useNavigate()
  const [courseId, setCourseId] = useState('')
  const [semesterId, setSemesterId] = useState('')
  const [comment, setComment] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(() => api.grade.listReviews({ page: 1, size: 20 }), [])

  /**
   * submitReview 把课程成绩提交到学校管理员审核流程。
   */
  const submitReview = useCallback(async () => {
    setSubmitting(true)
    setError(null)
    setMessage(null)
    try {
      await api.grade.submitReview({
        course_id: courseId,
        semester_id: semesterId || undefined,
        comment: comment || undefined,
      })
      setCourseId('')
      setSemesterId('')
      setComment('')
      setMessage('成绩审核已提交。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '成绩审核提交失败，请稍后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [comment, courseId, resource, semesterId])

  const columns = useMemo<TableColumn<GradeReview>[]>(() => [
    { key: 'course', title: '课程编号', dataIndex: 'course_id', priority: 'primary' },
    { key: 'semester', title: '学期', render: (row) => row.semester_id || '未指定' },
    { key: 'submitted', title: '提交时间', render: (row) => formatDateTime(row.submitted_at) },
    { key: 'status', title: '状态', render: (row) => <span className={styles.status}>{gradeReviewStatusLabel(row.status)}</span> },
    { key: 'locked', title: '锁定', render: (row) => (row.is_locked ? '已锁定' : '未锁定') },
    {
      key: 'actions',
      title: '操作',
      render: () => (
        <Button variant="outline" size="sm" icon={<Calculator size={14} />} onClick={() => navigate('/teacher/grades/details')}>
          查看明细
        </Button>
      ),
    },
  ], [navigate])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Send size={28} />课程期末成绩报送</h1>
          <p className={styles.subtitle}>按课程提交成绩审核，审核通过后由成绩中心锁定发布。</p>
        </div>
        <div className={styles.actions}>
          <Button variant="outline" icon={<HelpCircle size={16} />} onClick={() => navigate('/teacher/grades/appeals')}>处理申诉</Button>
          <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
        </div>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="提交成功">{message}</Callout>}

      <section className={styles.panel}>
        <h2>提交审核</h2>
        <div className={styles.grid}>
          <label className={styles.field}>课程编号<Input fullWidth value={courseId} onChange={(event) => setCourseId(event.target.value)} /></label>
          <label className={styles.field}>学期编号<Input fullWidth value={semesterId} onChange={(event) => setSemesterId(event.target.value)} /></label>
        </div>
        <label className={styles.field}>提交说明<Textarea value={comment} onChange={(event) => setComment(event.target.value)} /></label>
        <Button icon={<Send size={16} />} loading={submitting} onClick={submitReview}>提交成绩审核</Button>
      </section>

      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取成绩报送记录" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无报送记录" emptyDescription="当前还没有课程成绩报送记录。" ariaLabel="教师成绩报送记录" />
        </div>
      )}
    </div>
  )
}

export default TeacherGradesPage
