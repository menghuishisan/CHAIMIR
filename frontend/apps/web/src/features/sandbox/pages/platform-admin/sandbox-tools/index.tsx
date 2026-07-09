// SandboxToolsPage 展示平台沙箱工具定义，数据来自 sandbox 后端模块。

import React, { useMemo } from 'react'
import type { SandboxToolDefinition } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table } from '@chaimir/ui'
import { Package, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../../../admin/pages/list.module.css'
import { sandboxToolKindLabel, toolStatusLabel } from '../../../../../utils/index'

/**
 * SandboxToolsPage 读取全局沙箱工具定义。
 */
const SandboxToolsPage: React.FC = () => {
  const resource = useAsyncResource(() => api.sandbox.listTools(), [])
  const rows = resource.data || []

  const columns = useMemo<TableColumn<SandboxToolDefinition>[]>(() => [
    { key: 'name', title: '工具名称', dataIndex: 'name', priority: 'primary' },
    { key: 'code', title: '工具编码', dataIndex: 'code', priority: 'secondary' },
    { key: 'kind', title: '工具类型', render: (row) => sandboxToolKindLabel(row.kind) },
    {
      key: 'ecos',
      title: '适用生态',
      render: (row) => row.eco_tags.join('、') || '通用',
    },
    {
      key: 'status',
      title: '全局状态',
      render: (row) => <span className={styles.status}>{toolStatusLabel(row.status)}</span>,
    },
  ], [])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Package className={styles.icon} size={28} />
            全局沙箱工具链
          </h1>
          <p className={styles.subtitle}>查看平台登记的 Web 工具和受控命令工具。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <LoadingState title="正在获取沙箱工具" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="id"
            emptyTitle="暂无工具"
            emptyDescription="当前平台还没有登记沙箱工具。"
            ariaLabel="平台沙箱工具列表"
          />
        </div>
      )}
    </div>
  )
}

export default SandboxToolsPage
