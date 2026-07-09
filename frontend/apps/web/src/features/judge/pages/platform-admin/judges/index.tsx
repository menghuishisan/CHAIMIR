// JudgesPage 展示平台判题器配置列表，数据来自 judge 后端模块。

import React, { useMemo } from 'react'
import type { Judger } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table } from '@chaimir/ui'
import { Cpu, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../../../admin/pages/list.module.css'
import { formatSeconds, judgerStatusLabel, judgerTypeLabel } from '../../../../../utils/index'


/**
 * JudgesPage 读取判题器声明和执行器状态。
 */
const JudgesPage: React.FC = () => {
  const resource = useAsyncResource(() => api.judge.listJudgers(), [])
  const rows = resource.data || []

  const columns = useMemo<TableColumn<Judger>[]>(() => [
    { key: 'name', title: '判题器名称', dataIndex: 'name', priority: 'primary' },
    { key: 'code', title: '编码', dataIndex: 'code', priority: 'secondary' },
    { key: 'type', title: '类型', render: (row) => judgerTypeLabel(row.type) },
    {
      key: 'runtime',
      title: '需要运行时',
      render: (row) => (row.runtime_required ? '需要' : '不需要'),
    },
    {
      key: 'timeout',
      title: '默认超时',
      render: (row) => formatSeconds(row.default_timeout_sec),
    },
    {
      key: 'status',
      title: '状态',
      render: (row) => <span className={styles.status}>{judgerStatusLabel(row.status)}</span>,
    },
  ], [])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Cpu className={styles.icon} size={28} />
            判题引擎集群
          </h1>
          <p className={styles.subtitle}>查看平台判题器配置、运行时依赖和默认执行约束。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <LoadingState title="正在获取判题器" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="id"
            emptyTitle="暂无判题器"
            emptyDescription="当前平台还没有登记判题器。"
            ariaLabel="平台判题器列表"
          />
        </div>
      )}
    </div>
  )
}

export default JudgesPage
