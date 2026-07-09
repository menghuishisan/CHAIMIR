// SchoolsPage 展示平台租户列表，数据来自 admin 平台租户接口。

import React, { useMemo } from 'react'
import type { TenantSummary } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table } from '@chaimir/ui'
import { Building, Eye, Gauge, RefreshCw } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../../../admin/pages/list.module.css'
import { deployModeLabel, formatDateTime, tenantStatusLabel } from '../../../../../utils/index'



/**
 * SchoolsPage 读取平台租户摘要并提供详情、配额入口。
 */
const SchoolsPage: React.FC = () => {
  const navigate = useNavigate()
  const resource = useAsyncResource(() => api.admin.listTenants(), [])
  const rows = resource.data || []

  const columns = useMemo<TableColumn<TenantSummary>[]>(() => [
    { key: 'name', title: '学校名称', dataIndex: 'name', priority: 'primary' },
    { key: 'code', title: '代号', dataIndex: 'code', priority: 'secondary' },
    {
      key: 'status',
      title: '运行状态',
      render: (row) => <span className={styles.status}>{tenantStatusLabel(row.status)}</span>,
    },
    {
      key: 'deployMode',
      title: '部署形态',
      render: (row) => deployModeLabel(row.deploy_mode),
    },
    {
      key: 'expireAt',
      title: '到期时间',
      render: (row) => <span className={styles.muted}>{formatDateTime(row.expire_at)}</span>,
    },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <div className={styles.actions}>
          <Button
            variant="outline"
            size="sm"
            icon={<Gauge size={14} />}
            onClick={() => navigate(`/platform-admin/schools/${row.tenant_id}/quotas`)}
          >
            配额
          </Button>
          <Button
            variant="secondary"
            size="sm"
            icon={<Eye size={14} />}
            onClick={() => navigate(`/platform-admin/schools/${row.tenant_id}`)}
          >
            详情
          </Button>
        </div>
      ),
    },
  ], [navigate])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Building className={styles.icon} size={28} />
            租户学校管理
          </h1>
          <p className={styles.subtitle}>管理已入驻学校、部署形态和资源入口。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <LoadingState title="正在获取学校列表" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="tenant_id"
            emptyTitle="暂无学校"
            emptyDescription="当前还没有已入驻的学校租户。"
            ariaLabel="平台学校租户列表"
          />
        </div>
      )}
    </div>
  )
}

export default SchoolsPage
