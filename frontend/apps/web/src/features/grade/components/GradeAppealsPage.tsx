// GradeAppealsPage 是教师与学校管理员共用的成绩申诉处理页面。

import React, { useMemo, useState } from 'react'
import type { GradeAppeal } from '@chaimir/api-client'
import { GradeAppealStatus } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, ResourceState, Select, Table } from '@chaimir/ui'
import { RefreshCw, Scale } from 'lucide-react'
import { api } from '../../../app/api'
import { useAsyncResource } from '../../../hooks'
import { gradeAppealStatusFilterOptions, gradeAppealStatusLabel } from '../../../utils'
import { userFacingErrorMessage } from '../../../utils/userFacingError'
import styles from '../pages/grade.module.css'
import { GradeDecisionDialog } from './GradeDecisionDialog'

interface GradeAppealsPageProps {
  title: string
  subtitle: string
  ariaLabel: string
}

/** GradeAppealsPage 展示并处理当前角色有权访问的申诉工单。 */
export function GradeAppealsPage({ title, subtitle, ariaLabel }: GradeAppealsPageProps): React.ReactElement {
  const [status, setStatus] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const [decision, setDecision] = useState<{ appeal: GradeAppeal; action: 'accept' | 'reject' } | null>(null)
  const [comment, setComment] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const resource = useAsyncResource(() => api.grade.listAppeals({
    status: status ? Number(status) as GradeAppealStatus : undefined,
    page: 1,
    size: 20,
  }), [status])

  /** submitDecision 将审核结论提交给服务端并刷新列表。 */
  const submitDecision = async () => {
    if (!decision || !comment.trim()) return
    setSubmitting(true)
    setError('')
    setMessage('')
    try {
      const payload = { comment: comment.trim() }
      if (decision.action === 'accept') await api.grade.acceptAppeal(decision.appeal.id, payload)
      else await api.grade.rejectAppeal(decision.appeal.id, payload)
      setMessage(decision.action === 'accept' ? '申诉已受理。' : '申诉已驳回。')
      setDecision(null)
      setComment('')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '申诉处理失败，请稍后重试。'))
    } finally {
      setSubmitting(false)
    }
  }

  const columns = useMemo<TableColumn<GradeAppeal>[]>(() => [
    { key: 'request', title: '申诉', render: () => '成绩申诉', priority: 'primary' },
    { key: 'reason', title: '申诉理由', dataIndex: 'reason', priority: 'primary' },
    { key: 'status', title: '状态', render: (row) => <span className={styles.status}>{gradeAppealStatusLabel(row.status)}</span> },
    {
      key: 'actions',
      title: '操作',
      render: (row) => row.status === GradeAppealStatus.PENDING ? (
        <div className={styles.actions}>
          <Button variant="outline" size="sm" onClick={() => setDecision({ appeal: row, action: 'accept' })}>受理</Button>
          <Button variant="ghost" size="sm" onClick={() => setDecision({ appeal: row, action: 'reject' })}>驳回</Button>
        </div>
      ) : <span className={styles.muted}>无需操作</span>,
    },
  ], [])

  const rows = resource.data?.list || []
  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Scale size={28} />{title}</h1>
          <p className={styles.subtitle}>{subtitle}</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="操作成功">{message}</Callout>}
      <div className={styles.toolbar}><Select value={status} options={gradeAppealStatusFilterOptions} onChange={setStatus} /></div>
      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取申诉工单" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无申诉" emptyDescription="当前没有成绩申诉工单。" ariaLabel={ariaLabel} />
        </div>
      )}
      <GradeDecisionDialog open={Boolean(decision)} title={decision?.action === 'accept' ? '受理成绩申诉' : '驳回成绩申诉'} description={decision?.action === 'accept' ? '请说明已核验的材料和后续复核安排。' : '请明确说明驳回原因及学生可采取的下一步。'} confirmLabel={decision?.action === 'accept' ? '确认受理' : '确认驳回'} danger={decision?.action === 'reject'} value={comment} loading={submitting} onChange={setComment} onClose={() => { setDecision(null); setComment('') }} onConfirm={() => void submitDecision()} />
    </div>
  )
}
