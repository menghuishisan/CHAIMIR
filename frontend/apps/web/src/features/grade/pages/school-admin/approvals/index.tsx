// ApprovalsPage 展示成绩审核列表，并调用 grade 后端审批接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { GradeReview } from '@chaimir/api-client'
import { GradeReviewStatus } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Select, Table, useConfirm, ResourceState } from '@chaimir/ui'
import { CheckCircle, LockOpen, RefreshCw, XCircle } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useActionFeedback, useAsyncResource } from '../../../../../hooks'
import styles from '../../grade.module.css'
import { gradeReviewStatusFilterOptions, gradeReviewStatusLabel } from '../../../../../utils/index'
import { GradeDecisionDialog } from '../../../components/GradeDecisionDialog'


const ApprovalsPage: React.FC = () => {
  const confirm = useConfirm()
  const [status, setStatus] = useState('')
  const [decision, setDecision] = useState<{ review: GradeReview; action: 'reject' | 'unlock' } | null>(null)
  const [comment, setComment] = useState('')
  const resource = useAsyncResource(() => api.grade.listReviews({
    status: status ? Number(status) as GradeReviewStatus : undefined,
    page: 1,
    size: 20,
  }), [status])
  const { error, message, pendingAction, runAction } = useActionFeedback(resource.reload, '审核操作失败，请稍后重试。')

  /** approveReview 在锁定并发布成绩前说明影响。 */
  const approveReview = useCallback(async (review: GradeReview) => {
    const confirmed = await confirm({ title: '通过成绩审核', description: '通过后本学期课程成绩将发布并锁定，后续修改需要先解锁。', confirmLabel: '确认通过', confirmVariant: 'primary' })
    if (confirmed) await runAction(`${review.id}-approve`, () => api.grade.approveReview(review.id, { semester_id: review.semester_id }), '成绩审核已通过。')
  }, [confirm, runAction])

  /** submitDecision 使用管理员填写的实际意见退回或解锁成绩。 */
  const submitDecision = async () => {
    if (!decision || !comment.trim()) return
    const completed = await runAction(
      `${decision.review.id}-${decision.action}`,
      () => decision.action === 'reject'
        ? api.grade.rejectReview(decision.review.id, { semester_id: decision.review.semester_id, comment: comment.trim() })
        : api.grade.unlockReview(decision.review.id, { semester_id: decision.review.semester_id, comment: comment.trim() }),
      decision.action === 'reject' ? '成绩审核已驳回。' : '成绩审核已解锁。',
    )
    if (completed) {
      setDecision(null)
      setComment('')
    }
  }

  const columns = useMemo<TableColumn<GradeReview>[]>(() => [
    { key: 'review', title: '成绩报送', render: () => '课程成绩', priority: 'primary' },
    { key: 'semester', title: '学期', render: (row) => row.semester_id ? '已指定' : '未指定' },
    { key: 'status', title: '状态', render: (row) => <span className={styles.status}>{gradeReviewStatusLabel(row.status)}</span> },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <div className={styles.actions}>
          {row.status === GradeReviewStatus.PENDING && <Button variant="outline" size="sm" icon={<CheckCircle size={14} />} loading={pendingAction === `${row.id}-approve`} disabled={!row.semester_id || Boolean(pendingAction)} onClick={() => approveReview(row)}>通过</Button>}
          {row.status === GradeReviewStatus.PENDING && <Button variant="outline" size="sm" icon={<XCircle size={14} />} disabled={Boolean(pendingAction)} onClick={() => setDecision({ review: row, action: 'reject' })}>驳回</Button>}
          {row.status === GradeReviewStatus.APPROVED && row.is_locked && <Button variant="ghost" size="sm" icon={<LockOpen size={14} />} disabled={Boolean(pendingAction)} onClick={() => setDecision({ review: row, action: 'unlock' })}>解锁</Button>}
          {row.status === GradeReviewStatus.REJECTED && <span className={styles.muted}>无需操作</span>}
        </div>
      ),
    },
  ], [approveReview, pendingAction])

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
      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取成绩审核" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无审核" emptyDescription="当前没有成绩审核记录。" ariaLabel="成绩审核列表" />
        </div>
      )}
      <GradeDecisionDialog open={Boolean(decision)} title={decision?.action === 'reject' ? '驳回成绩审核' : '解锁已发布成绩'} description={decision?.action === 'reject' ? '请说明需要教师修正的具体问题。' : '请说明解锁原因和后续核验安排。'} confirmLabel={decision?.action === 'reject' ? '确认驳回' : '确认解锁'} danger={decision?.action === 'reject'} value={comment} loading={Boolean(pendingAction)} onChange={setComment} onClose={() => { setDecision(null); setComment('') }} onConfirm={() => void submitDecision()} />
    </div>
  )
}

export default ApprovalsPage
