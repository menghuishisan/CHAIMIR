// AuditPage 展示平台审计流水，数据来自 admin 审计接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError, AuditLogEntry } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table } from '@chaimir/ui'
import { Download, FileText, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../list.module.css'
import { formatDateTime } from '../../../../../utils/index'

const PAGE_SIZE = 20


/**
 * AuditPage 读取审计流水并支持创建导出任务。
 */
const AuditPage: React.FC = () => {
  const resource = useAsyncResource(
    () => api.admin.queryAudit({ page: 1, size: PAGE_SIZE }),
    []
  )
  const rows = resource.data?.list || []
  const [exporting, setExporting] = useState(false)
  const [exportMessage, setExportMessage] = useState<string | null>(null)
  const [exportError, setExportError] = useState<ApiError | null>(null)

  const handleExport = useCallback(async () => {
    setExportError(null)
    setExportMessage(null)
    setExporting(true)
    try {
      const task = await api.admin.exportAudit({ page: 1, size: PAGE_SIZE })
      setExportMessage(`导出任务已创建：${task.subject}`)
    } catch (error) {
      setExportError(error as ApiError)
    } finally {
      setExporting(false)
    }
  }, [])

  const columns = useMemo<TableColumn<AuditLogEntry>[]>(() => [
    {
      key: 'createdAt',
      title: '时间',
      render: (row) => <span className={styles.muted}>{formatDateTime(row.created_at)}</span>,
      priority: 'primary',
    },
    { key: 'actor', title: '操作人', dataIndex: 'actor_id', priority: 'secondary' },
    { key: 'action', title: '动作', dataIndex: 'action' },
    { key: 'target', title: '对象类型', dataIndex: 'target_type' },
    {
      key: 'ip',
      title: 'IP 地址',
      render: (row) => row.ip || '未记录',
    },
    {
      key: 'trace',
      title: '追踪编号',
      render: (row) => row.trace_id || '未记录',
    },
  ], [])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <FileText className={styles.icon} size={28} />
            超管动作防篡改流水
          </h1>
          <p className={styles.subtitle}>查看平台级管理操作与追踪编号。</p>
        </div>
        <div className={styles.toolbar}>
          <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
            刷新
          </Button>
          <Button
            variant="secondary"
            icon={<Download size={16} />}
            loading={exporting}
            onClick={handleExport}
          >
            创建导出任务
          </Button>
        </div>
      </div>

      {exportMessage && <div className={styles.status}>{exportMessage}</div>}
      {exportError && (
        <ErrorState error={exportError} onRetry={() => setExportError(null)} title="导出任务创建失败" />
      )}
      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <LoadingState title="正在获取审计流水" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="id"
            emptyTitle="暂无审计记录"
            emptyDescription="当前没有可展示的审计流水。"
            ariaLabel="平台审计流水列表"
          />
        </div>
      )}
    </div>
  )
}

export default AuditPage
