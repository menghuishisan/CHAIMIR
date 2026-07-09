// BackupsPage 展示平台备份记录，数据来自 admin 备份接口。

import React, { useMemo } from 'react'
import type { BackupRecord } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table } from '@chaimir/ui'
import { RefreshCw, Save } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../list.module.css'
import { backupStatusLabel, backupTypeLabel, formatBytes, formatDateTime } from '../../../../../utils/index'

const PAGE_SIZE = 20



/**
 * BackupsPage 读取平台备份任务记录。
 */
const BackupsPage: React.FC = () => {
  const resource = useAsyncResource(
    () => api.admin.listBackups({ page: 1, size: PAGE_SIZE }),
    []
  )
  const rows = resource.data?.list || []

  const columns = useMemo<TableColumn<BackupRecord>[]>(() => [
    { key: 'id', title: '备份编号', dataIndex: 'id', priority: 'primary' },
    { key: 'type', title: '触发类型', render: (row) => backupTypeLabel(row.type), priority: 'secondary' },
    {
      key: 'size',
      title: '体积',
      render: (row) => formatBytes(row.size_bytes),
    },
    {
      key: 'status',
      title: '状态',
      render: (row) => <span className={styles.status}>{backupStatusLabel(row.status)}</span>,
    },
    {
      key: 'startedAt',
      title: '开始时间',
      render: (row) => <span className={styles.muted}>{formatDateTime(row.started_at)}</span>,
    },
    {
      key: 'finishedAt',
      title: '完成时间',
      render: (row) => <span className={styles.muted}>{formatDateTime(row.finished_at)}</span>,
    },
  ], [])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Save className={styles.icon} size={28} />
            数据库冷备任务
          </h1>
          <p className={styles.subtitle}>查看平台受控运维任务写入的备份记录。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <LoadingState title="正在获取备份记录" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="id"
            emptyTitle="暂无备份"
            emptyDescription="当前还没有可展示的备份记录。"
            ariaLabel="平台备份记录列表"
          />
        </div>
      )}
    </div>
  )
}

export default BackupsPage
