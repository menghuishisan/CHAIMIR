// TeacherGradesAppealsPage 展示并处理教师可见的成绩申诉工单。

import React, { useCallback, useMemo, useState } from 'react'
import type { GradeAppeal } from '@chaimir/api-client'
import { GradeAppealStatus } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Select, Table } from '@chaimir/ui'
import { CheckCircle, HelpCircle, RefreshCw, XCircle } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../grade.module.css'
import { gradeAppealStatusFilterOptions, gradeAppealStatusLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'


const TeacherGradesAppealsPage: React.FC = () => {
  const [status, setStatus] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(() => api.grade.listAppeals({
    status: status ? Number(status) as GradeAppealStatus : undefined,
    page: 1,
    size: 20,
  }), [status])

  /**
   * handleAppeal 调用后端完成申诉受理或驳回。
   */
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
    { key: 'reason', title: '申诉理由', dataIndex: 'reason', priority: 'primary' },
    { key: 'status', title: '状态', render: (row) => <span className={styles.status}>{gradeAppealStatusLabel(row.status)}</span> },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <div className={styles.actions}>
          <Button variant="outline" size="sm" icon={<CheckCircle size={14} />} onClick={() => handleAppeal(() => api.grade.acceptAppeal(row.id, { comment: '申诉已进入复核流程。' }), '申诉已受理。')}>受理</Button>
          <Button variant="ghost" size="sm" icon={<XCircle size={14} />} onClick={() => handleAppeal(() => api.grade.rejectAppeal(row.id, { comment: '申诉材料不足。' }), '申诉已驳回。')}>驳回</Button>
        </div>
      ),
    },
  ], [handleAppeal])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><HelpCircle size={28} />成绩申诉工单复核</h1>
          <p className={styles.subtitle}>按后端申诉状态受理或驳回学生成绩申诉。</p>
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
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无申诉" emptyDescription="当前没有成绩申诉工单。" ariaLabel="教师成绩申诉列表" />
        </div>
      )}
    </div>
  )
}

export default TeacherGradesAppealsPage
