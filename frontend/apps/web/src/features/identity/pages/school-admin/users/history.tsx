// UserHistoryPage 展示账号导入批次历史，数据来自 identity 后端导入批次接口。

import React, { useMemo } from 'react'
import type { ImportBatch } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table } from '@chaimir/ui'
import { History, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../identity-admin.module.css'
import { formatDateTime, importBatchStatusLabel } from '../../../../../utils/index'


const UserHistoryPage: React.FC = () => {
  const resource = useAsyncResource(() => api.identity.listAccountImportBatches(), [])

  const columns = useMemo<TableColumn<ImportBatch>[]>(() => [
    { key: 'id', title: '批次编号', dataIndex: 'id', priority: 'primary' },
    { key: 'fileName', title: '文件名', dataIndex: 'file_name', priority: 'primary' },
    { key: 'total', title: '总数', dataIndex: 'total' },
    { key: 'success', title: '成功', dataIndex: 'success' },
    { key: 'failed', title: '失败', dataIndex: 'failed' },
    {
      key: 'status',
      title: '状态',
      render: (row) => <span className={styles.status}>{importBatchStatusLabel(row.status)}</span>,
    },
    {
      key: 'createdAt',
      title: '创建时间',
      render: (row) => <span className={styles.muted}>{formatDateTime(row.created_at)}</span>,
    },
  ], [])

  const rows = resource.data || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <History size={28} />
            历史导入批次
          </h1>
          <p className={styles.subtitle}>查看后端持久化的账号导入批次和执行结果。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取导入历史" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="id"
            emptyTitle="暂无导入历史"
            emptyDescription="当前还没有账号导入批次。"
            ariaLabel="账号导入批次列表"
          />
        </div>
      )}
    </div>
  )
}

export default UserHistoryPage
