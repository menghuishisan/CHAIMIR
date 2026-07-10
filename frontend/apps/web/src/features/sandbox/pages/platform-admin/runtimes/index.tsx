// RuntimesPage 展示平台链运行时列表，数据来自 sandbox 后端模块。

import React, { useMemo } from 'react'
import type { SandboxRuntime } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table } from '@chaimir/ui'
import { Eye, RefreshCw, Server } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../../../admin/pages/list.module.css'
import { runtimeSelftestStatusLabel, runtimeStatusLabel } from '../../../../../utils/index'

/**
 * RuntimesPage 读取链运行时声明和自检状态。
 */
const RuntimesPage: React.FC = () => {
  const navigate = useNavigate()
  const resource = useAsyncResource(() => api.sandbox.listRuntimes(), [])
  const rows = resource.data || []

  const columns = useMemo<TableColumn<SandboxRuntime>[]>(() => [
    { key: 'name', title: '运行时名称', dataIndex: 'name', priority: 'primary' },
    { key: 'code', title: '编码', dataIndex: 'code', priority: 'secondary' },
    { key: 'eco', title: '生态', dataIndex: 'eco' },
    {
      key: 'level',
      title: '适配等级',
      render: (row) => `L${row.adapter_level}`,
    },
    {
      key: 'status',
      title: '运行状态',
      render: (row) => <span className={styles.status}>{runtimeStatusLabel(row.status)}</span>,
    },
    {
      key: 'selftest',
      title: '自检状态',
      render: (row) => <span className={styles.muted}>{runtimeSelftestStatusLabel(row.selftest_status)}</span>,
    },
    {
      key: 'action',
      title: '操作',
      render: (row) => (
        <Button size="sm" variant="outline" icon={<Eye size={14} />} onClick={() => navigate(`/platform-admin/runtimes/${row.id}`)}>
          查看详情
        </Button>
      ),
    },
  ], [navigate])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Server className={styles.icon} size={28} />
            链运行时与镜像集
          </h1>
          <p className={styles.subtitle}>查看平台可用链运行时、适配等级和自检状态。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <LoadingState title="正在获取链运行时" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="id"
            emptyTitle="暂无运行时"
            emptyDescription="当前平台还没有登记链运行时。"
            ariaLabel="平台链运行时列表"
          />
        </div>
      )}
    </div>
  )
}

export default RuntimesPage
