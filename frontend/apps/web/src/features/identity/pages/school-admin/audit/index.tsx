// AuditPage 查询当前管理员可见审计日志，使用 identity 审计接口展示敏感操作流水。

import React, { useMemo, useState } from 'react'
import type { AuditLog } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Input, Table, ResourceState } from '@chaimir/ui'
import { FileText, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../identity-admin.module.css'
import { auditActionLabel, auditTargetLabel, formatDateTime } from '../../../../../utils/index'

const PAGE_SIZE = 20


const AuditPage: React.FC = () => {
  const [action, setAction] = useState('')
  const resource = useAsyncResource(() => api.identity.getAuditLogs({
    action: action || undefined,
    page: 1,
    size: PAGE_SIZE,
  }), [action])

  const columns = useMemo<TableColumn<AuditLog>[]>(() => [
    {
      key: 'createdAt',
      title: '时间',
      render: (row) => <span className={styles.muted}>{formatDateTime(row.created_at)}</span>,
      priority: 'primary',
    },
    { key: 'action', title: '操作类型', render: (row) => auditActionLabel(row.action), priority: 'secondary' },
    { key: 'target', title: '目标', render: (row) => auditTargetLabel(row.target_type) },
    { key: 'ip', title: 'IP 地址', render: (row) => row.ip || '暂无' },
    { key: 'trace', title: '追踪编号', render: (row) => row.trace_id || '暂无' },
  ], [])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <FileText size={28} />
            敏感操作流水
          </h1>
          <p className={styles.subtitle}>审计日志由 identity 模块统一记录，用于合规追溯。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      <div className={styles.toolbar}>
        <Input placeholder="按操作类型筛选" value={action} onChange={(event) => setAction(event.target.value)} />
      </div>

      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取审计日志" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="id"
            emptyTitle="暂无审计记录"
            emptyDescription="当前筛选条件下没有审计日志。"
            ariaLabel="审计日志列表"
          />
        </div>
      )}
    </div>
  )
}

export default AuditPage
