// AlertsPage 展示平台告警事件列表，数据来自 admin 告警事件接口。

import React, { useMemo } from 'react'
import type { AlertEvent } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table, ResourceState } from '@chaimir/ui'
import { BellRing, RefreshCw, Settings2 } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../list.module.css'
import { alertLevelLabel, formatDateTime, alertStatusLabel } from '../../../../../utils/index'

const PAGE_SIZE = 20



/**
 * AlertsPage 读取平台告警事件并提供规则配置入口。
 */
const AlertsPage: React.FC = () => {
  const navigate = useNavigate()
  const resource = useAsyncResource(
    () => api.admin.listAlertEvents({ page: 1, size: PAGE_SIZE }),
    []
  )
  const rows = resource.data?.list || []

  const columns = useMemo<TableColumn<AlertEvent>[]>(() => [
    {
      key: 'level',
      title: '级别',
      render: (row) => <span className={styles.status}>{alertLevelLabel(row.level)}</span>,
      priority: 'primary',
    },
    { key: 'message', title: '告警内容', dataIndex: 'message', priority: 'primary' },
    {
      key: 'status',
      title: '状态',
      render: (row) => alertStatusLabel(row.status),
    },
    {
      key: 'tenant',
      title: '告警范围',
      render: (row) => row.tenant_name || '平台范围',
    },
    {
      key: 'triggeredAt',
      title: '发生时间',
      render: (row) => <span className={styles.muted}>{formatDateTime(row.triggered_at)}</span>,
    },
  ], [])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <BellRing className={styles.icon} size={28} />
            平台告警事件
          </h1>
          <p className={styles.subtitle}>查看平台与租户范围内的待处理告警。</p>
        </div>
        <div className={styles.toolbar}>
          <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
            刷新
          </Button>
          <Button
            variant="secondary"
            icon={<Settings2 size={16} />}
            onClick={() => navigate('/platform-admin/alerts/rules')}
          >
            告警规则
          </Button>
        </div>
      </div>

      {resource.status === 'error' && (
        <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取告警事件" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="id"
            emptyTitle="暂无告警"
            emptyDescription="当前没有需要处理的告警事件。"
            ariaLabel="平台告警事件列表"
          />
        </div>
      )}
    </div>
  )
}

export default AlertsPage
