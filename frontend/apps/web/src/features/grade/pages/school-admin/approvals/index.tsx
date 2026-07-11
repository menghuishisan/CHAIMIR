// ApprovalsPage 展示成绩审核列表，并调用 grade 后端审批接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { GradeReview } from '@chaimir/api-client'
import { GradeReviewStatus } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Select, Table } from '@chaimir/ui'
import { CheckCircle, LockOpen, RefreshCw, XCircle } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../grade.module.css'
import { gradeReviewStatusFilterOptions, gradeReviewStatusLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'


const ApprovalsPage: React.FC = () => {
  const [status, setStatus] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(() => api.grade.listReviews({
    status: status ? Number(status) as GradeReviewStatus : undefined,
    page: 1,
    size: 20,
  }), [status])

  const reviewAction = useCallback(async (action: () => Promise<GradeReview>, successMessage: string) => {
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(successMessage)
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '审核操作失败，请稍后重试。'))
    }
  }, [resource])

  const columns = useMemo<TableColumn<GradeReview>[]>(() => [
    { key: 'course', title: '课程编号', dataIndex: 'course_id', priority: 'primary' },
    { key: 'submitter', title: '提交人', dataIndex: 'submitter_id' },
    { key: 'semester', title: '学期', render: (row) => row.semester_id || '未指定' },
    { key: 'status', title: '状态', render: (row) => <span className={styles.status}>{gradeReviewStatusLabel(row.status)}</span> },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <div className={styles.actions}>
          <Button variant="outline" size="sm" icon={<CheckCircle size={14} />} onClick={() => reviewAction(() => api.grade.approveReview(row.id, {}), '成绩审核已通过。')}>通过</Button>
          <Button variant="outline" size="sm" icon={<XCircle size={14} />} onClick={() => reviewAction(() => api.grade.rejectReview(row.id, { comment: '请教师修正后重新提交。' }), '成绩审核已驳回。')}>驳回</Button>
          {row.is_locked && <Button variant="ghost" size="sm" icon={<LockOpen size={14} />} onClick={() => reviewAction(() => api.grade.unlockReview(row.id, { semester_id: row.semester_id, comment: '学校管理员解锁后重新核验。' }), '成绩审核已解锁。')}>解锁</Button>}
        </div>
      ),
    },
  ], [reviewAction])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><CheckCircle size={28} />成绩发布审批</h1>
          <p className={styles.subtitle}>审核教师提交的课程成绩，审核通过后成绩锁定。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="操作成功">{message}</Callout>}
      <div className={styles.toolbar}><Select value={status} options={gradeReviewStatusFilterOptions} onChange={setStatus} /></div>
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取成绩审核" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无审核" emptyDescription="当前没有成绩审核记录。" ariaLabel="成绩审核列表" />
        </div>
      )}
    </div>
  )
}

export default ApprovalsPage
