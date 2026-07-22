// AppealsPage 展示成绩申诉列表，并调用 grade 后端处理接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { GradeAppeal } from '@chaimir/api-client'
import { GradeAppealStatus } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Select, Table } from '@chaimir/ui'
import { RefreshCw, Scale } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../grade.module.css'
import { gradeAppealStatusFilterOptions, gradeAppealStatusLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const AppealsPage: React.FC = () => {
  const [status, setStatus] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(() => api.grade.listAppeals({
    status: status ? Number(status) as GradeAppealStatus : undefined,
    page: 1,
    size: 20,
  }), [status])

  const handleAppeal = useCallback(async (action: () => Promise<GradeAppeal>, successMessage: string) => {
    setError(null)
    setMessage(null)
    try {
      await action()
      setMessage(successMessage)
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '申诉处理失败，请稍后重试。'))
    }
  }, [resource])

  const columns = useMemo<TableColumn<GradeAppeal>[]>(() => [
    { key: 'student', title: '学生编号', dataIndex: 'student_id', priority: 'primary' },
    { key: 'course', title: '课程编号', dataIndex: 'course_id' },
    { key: 'reason', title: '理由', dataIndex: 'reason', priority: 'primary' },
    { key: 'status', title: '状态', render: (row) => <span className={styles.status}>{gradeAppealStatusLabel(row.status)}</span> },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <div className={styles.actions}>
          {row.status === GradeAppealStatus.PENDING && <Button variant="outline" size="sm" onClick={() => handleAppeal(() => api.grade.acceptAppeal(row.id, { comment: '申诉已进入复核流程。' }), '申诉已受理。')}>受理</Button>}
          {row.status === GradeAppealStatus.PENDING && <Button variant="ghost" size="sm" onClick={() => handleAppeal(() => api.grade.rejectAppeal(row.id, { comment: '申诉材料不足。' }), '申诉已驳回。')}>驳回</Button>}
          {row.status !== GradeAppealStatus.PENDING && <span className={styles.muted}>无需操作</span>}
        </div>
      ),
    },
  ], [handleAppeal])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Scale size={28} />学生申诉工单</h1>
          <p className={styles.subtitle}>查看申诉进度，并受理或驳回待处理申请。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="操作成功">{message}</Callout>}
      <div className={styles.toolbar}><Select value={status} options={gradeAppealStatusFilterOptions} onChange={setStatus} /></div>
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取申诉工单" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无申诉" emptyDescription="当前没有成绩申诉工单。" ariaLabel="成绩申诉列表" />
        </div>
      )}
    </div>
  )
}

export default AppealsPage
