// AlertsPage 展示当前学生可见的学业预警，并调用后端确认预警。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError, GradeWarning } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Table } from '@chaimir/ui'
import { AlertTriangle, CheckCircle, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../grade.module.css'
import { formatDateTime, gradeWarningDetailLabel, gradeWarningStatusLabel, gradeWarningTypeLabel } from '../../../../../utils/index'


const AlertsPage: React.FC = () => {
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(async () => {
    const me = await api.identity.getMe()
    return api.grade.listWarnings({ student_id: me.account.id, page: 1, size: 20 })
  }, [])

  /**
   * ackWarning 确认学生已阅读预警。
   */
  const ackWarning = useCallback(async (id: string) => {
    setError(null)
    setMessage(null)
    try {
      await api.grade.ackWarning(id)
      setMessage('预警已确认。')
      resource.reload()
    } catch (actionError) {
      setError((actionError as ApiError).message || '预警确认失败，请稍后重试。')
    }
  }, [resource])

  const columns = useMemo<TableColumn<GradeWarning>[]>(() => [
    { key: 'type', title: '预警类型', render: (row) => <span className={styles.status}>{gradeWarningTypeLabel(row.type)}</span>, priority: 'primary' },
    { key: 'semester', title: '学期', dataIndex: 'semester_id' },
    { key: 'detail', title: '触发说明', render: (row) => gradeWarningDetailLabel(row.detail) },
    { key: 'created', title: '下发时间', render: (row) => formatDateTime(row.created_at) },
    { key: 'status', title: '状态', render: (row) => gradeWarningStatusLabel(row.status) },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <Button variant="outline" size="sm" icon={<CheckCircle size={14} />} onClick={() => ackWarning(row.id)} disabled={row.status !== 1}>
          确认
        </Button>
      ),
    },
  ], [ackWarning])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><AlertTriangle size={28} />学业预警</h1>
          <p className={styles.subtitle}>查看学校发布的预警并确认已阅读。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="操作成功">{message}</Callout>}
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取学业预警" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无预警" emptyDescription="当前没有需要处理的学业预警。" ariaLabel="学生学业预警列表" />
        </div>
      )}
    </div>
  )
}

export default AlertsPage
