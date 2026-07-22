// SystemAlertsPage 展示学校管理员可见的系统告警事件，并调用 admin 后端处理接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { AlertEvent } from '@chaimir/api-client'
import { AlertStatus } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Select, Table } from '@chaimir/ui'
import { BellRing, CheckCircle, RefreshCw, XCircle } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../list.module.css'
import { alertStatusFilterOptions, formatDateTime, alertStatusLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const SystemAlertsPage: React.FC = () => {
  const [status, setStatus] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(() => api.admin.listAlertEvents({
    status: status ? Number(status) as AlertStatus : undefined,
    page: 1,
    size: 20,
  }), [status])

  /**
   * handleEvent 按后端状态机处理或忽略告警事件。
   */
  const handleEvent = useCallback(async (eventId: string, nextStatus: AlertStatus, successMessage: string) => {
    setError(null)
    setMessage(null)
    try {
      await api.admin.handleAlertEvent(eventId, { status: nextStatus })
      setMessage(successMessage)
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '告警处理失败，请稍后重试。'))
    }
  }, [resource])

  const columns = useMemo<TableColumn<AlertEvent>[]>(() => [
    { key: 'level', title: '级别', render: (row) => <span className={styles.status}>L{row.level}</span> },
    { key: 'rule', title: '规则编号', dataIndex: 'rule_id' },
    { key: 'message', title: '告警描述', dataIndex: 'message', priority: 'primary' },
    { key: 'status', title: '状态', render: (row) => alertStatusLabel(row.status) },
    { key: 'time', title: '触发时间', render: (row) => formatDateTime(row.triggered_at) },
    {
      key: 'actions',
      title: '处理动作',
      render: (row) => (
        <div className={styles.actions}>
          <Button variant="outline" size="sm" icon={<CheckCircle size={14} />} onClick={() => handleEvent(row.id, AlertStatus.HANDLED, '告警已标记为已处理。')} disabled={row.status !== AlertStatus.PENDING}>处理</Button>
          <Button variant="ghost" size="sm" icon={<XCircle size={14} />} onClick={() => handleEvent(row.id, AlertStatus.IGNORED, '告警已忽略。')} disabled={row.status !== AlertStatus.PENDING}>忽略</Button>
        </div>
      ),
    },
  ], [handleEvent])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><BellRing className={styles.icon} />系统级告警</h1>
          <p className={styles.subtitle}>监控本校资源和平台运行状态，及时处理待办告警。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {error && <div className={styles.muted}>{error}</div>}
      {message && <Callout variant="success" title="操作成功">{message}</Callout>}
      <div className={styles.toolbar}><Select value={status} options={alertStatusFilterOptions} onChange={setStatus} /></div>
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取系统告警" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="id" emptyTitle="暂无告警" emptyDescription="当前没有系统级告警事件。" ariaLabel="学校系统告警列表" />
        </div>
      )}
    </div>
  )
}

export default SystemAlertsPage
