// StatisticsPage 是平台与学校运营统计页面的唯一实现。

import React, { useMemo } from 'react'
import type { Statistics } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, ResourceState, Table } from '@chaimir/ui'
import { ArrowLeft, LineChart, RefreshCw } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { useAsyncResource } from '../../../hooks'
import { formatMetricsSummary, recentDateRange } from '../../../utils'
import styles from '../pages/dashboard.module.css'

interface StatisticsPageProps {
  title: string
  loadingTitle: string
  emptyDescription: string
  ariaLabel: string
  load: (range: { from: string; to: string }) => Promise<Statistics[]>
  backPath?: string
}

/** StatisticsPage 读取并展示最近三十天的运营统计。 */
export function StatisticsPage({ title, loadingTitle, emptyDescription, ariaLabel, load, backPath }: StatisticsPageProps): React.ReactElement {
  const navigate = useNavigate()
  const range = useMemo(() => recentDateRange(30), [])
  const resource = useAsyncResource(() => load(range), [load, range.from, range.to])
  const columns = useMemo<TableColumn<Statistics>[]>(() => [
    { key: 'date', title: '日期', dataIndex: 'date', priority: 'primary' },
    { key: 'metrics', title: '指标摘要', render: (row) => formatMetricsSummary(row.metrics), priority: 'secondary' },
  ], [])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}><LineChart className={styles.icon} size={28} />{title}</h1>
        <div className={styles.headerActions}>
          {backPath && <Button variant="ghost" icon={<ArrowLeft size={16} />} onClick={() => navigate(backPath)}>返回看板</Button>}
          <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
        </div>
      </div>
      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title={loadingTitle} />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <section className={styles.panel}>
          <h2 className={styles.panelTitle}>近 30 天统计快照</h2>
          <Table columns={columns} rows={resource.data || []} rowKey={(row) => `${row.scope}-${row.date}`} emptyTitle="暂无统计" emptyDescription={emptyDescription} ariaLabel={ariaLabel} />
        </section>
      )}
    </div>
  )
}
