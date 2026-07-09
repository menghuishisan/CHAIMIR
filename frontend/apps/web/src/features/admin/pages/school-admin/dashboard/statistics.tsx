// AdminStatisticsPage 展示学校统计快照，数据来自 admin 学校统计接口。

import React, { useMemo } from 'react'
import type { Statistics } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table } from '@chaimir/ui'
import { LineChart, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../dashboard.module.css'
import { formatMetricsSummary, recentDateRange } from '../../../../../utils/index'

/**
 * AdminStatisticsPage 读取本校近 30 天统计快照。
 */
const AdminStatisticsPage: React.FC = () => {
  const range = useMemo(() => recentDateRange(30), [])
  const resource = useAsyncResource(
    () => api.admin.getSchoolStatistics(range),
    [range.from, range.to]
  )
  const rows = resource.data || []

  const columns = useMemo<TableColumn<Statistics>[]>(() => [
    { key: 'date', title: '日期', dataIndex: 'date', priority: 'primary' },
    {
      key: 'metrics',
      title: '指标摘要',
      render: (row) => formatMetricsSummary(row.metrics),
      priority: 'secondary',
    },
  ], [])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <LineChart className={styles.icon} size={28} />
          深度统计报表
        </h1>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <LoadingState title="正在获取统计报表" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <section className={styles.panel}>
          <h2 className={styles.panelTitle}>近 30 天统计快照</h2>
          <Table
            columns={columns}
            rows={rows}
            rowKey={(row) => `${row.scope}-${row.tenant_id || 'current'}-${row.date}`}
            emptyTitle="暂无统计"
            emptyDescription="当前区间没有可展示的统计快照。"
            ariaLabel="学校统计快照列表"
          />
        </section>
      )}
    </div>
  )
}

export default AdminStatisticsPage
