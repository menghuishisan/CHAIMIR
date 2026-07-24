// TeacherGradesPage 展示教师成绩报送记录，并通过后端提交新的成绩审核。

import React, { useCallback, useMemo, useState } from 'react'
import type { GradeReview } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Select, Table, Textarea, ResourceState, FormField } from '@chaimir/ui'
import { Calculator, HelpCircle, RefreshCw, Send } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
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
  const resource = useAsyncResource(() => api.grade.listOwnReviews({ page: 1, size: 20 }), [])
  const formOptions = useAsyncResource(async () => {
    const [courses, semesters] = await Promise.all([
      api.teaching.getCourses({ role: 'teacher', page: 1, size: 100 }),
      api.grade.listSemesters(),
    ])
    return { courses: courses.list, semesters }
  }, [])

  /**
   * submitReview 把课程成绩提交到学校管理员审核流程。
   */
  const submitReview = useCallback(async () => {
    setSubmitting(true)
    setError(null)
    setMessage(null)
    try {
      if (!courseId || !semesterId) {
        setError('请选择课程和学期。')
        return
      }
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
    { key: 'course', title: '课程', dataIndex: 'course_name', priority: 'primary' },
    { key: 'semester', title: '学期', render: (row) => row.semester_id ? '已指定' : '未指定' },
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
        {formOptions.status === 'error' && <div className={styles.error} role="alert">课程或学期暂时无法读取，请刷新后重试。</div>}
        <div className={styles.grid}>
          <FormField className={styles.field} label="课程"><Select fullWidth value={courseId} placeholder="请选择课程" options={(formOptions.data?.courses || []).map((course) => ({ value: course.id, label: course.name }))} onChange={setCourseId} /></FormField>
          <FormField className={styles.field} label="学期"><Select fullWidth value={semesterId} placeholder="请选择学期" options={(formOptions.data?.semesters || []).map((semester) => ({ value: semester.id, label: semester.name }))} onChange={setSemesterId} /></FormField>
        </div>
        <FormField className={styles.field} label="提交说明"><Textarea value={comment} onChange={(event) => setComment(event.target.value)} /></FormField>
        <Button icon={<Send size={16} />} loading={submitting} disabled={formOptions.status === 'loading'} onClick={submitReview}>提交成绩审核</Button>
      </section>

      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取成绩报送记录" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无报送记录" emptyDescription="当前还没有课程成绩报送记录。" ariaLabel="教师成绩报送记录" />
        </div>
      )}
    </div>
  )
}

export default TeacherGradesPage
