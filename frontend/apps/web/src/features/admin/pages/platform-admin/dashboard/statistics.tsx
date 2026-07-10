// PlatformStatisticsPage 展示平台近 30 天运营统计快照。

import React, { useMemo } from 'react'
import type { Statistics } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table } from '@chaimir/ui'
import { ArrowLeft, LineChart, RefreshCw } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import { formatMetricsSummary, recentDateRange } from '../../../../../utils/index'
import styles from '../../dashboard.module.css'

/**
 * PlatformStatisticsPage 读取平台级按日统计，并保持原始时间序列可扫描。
 */
const PlatformStatisticsPage: React.FC = () => {
  const navigate = useNavigate()
  const range = useMemo(() => recentDateRange(30), [])
  const resource = useAsyncResource(
    () => api.admin.getPlatformStatistics(range),
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
          平台运营统计
        </h1>
        <div className={styles.headerActions}>
          <Button variant="ghost" icon={<ArrowLeft size={16} />} onClick={() => navigate('/platform-admin/dashboard')}>返回看板</Button>
          <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
        </div>
      </div>

      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取平台统计" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <section className={styles.panel}>
          <h2 className={styles.panelTitle}>近 30 天统计快照</h2>
          <Table
            columns={columns}
            rows={rows}
            rowKey={(row) => `${row.scope}-platform-${row.date}`}
            emptyTitle="暂无统计"
            emptyDescription="当前区间没有可展示的平台统计快照。"
            ariaLabel="平台统计快照列表"
          />
        </section>
      )}
    </div>
  )
}

export default PlatformStatisticsPage
