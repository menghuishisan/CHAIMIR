// ApplicationsPage 展示学校入驻申请列表，数据来自 admin 平台申请接口。

import React, { useMemo } from 'react'
import type { TenantApplicationSummary } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Table } from '@chaimir/ui'
import { Eye, Inbox, RefreshCw } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../../../admin/pages/list.module.css'
import { formatDateTime, tenantApplicationStatusLabel } from '../../../../../utils/index'



/**
 * ApplicationsPage 读取学校入驻申请并提供处理入口。
 */
const ApplicationsPage: React.FC = () => {
  const navigate = useNavigate()
  const resource = useAsyncResource(() => api.admin.listApplications(), [])
  const rows = resource.data || []

  const columns = useMemo<TableColumn<TenantApplicationSummary>[]>(() => [
    { key: 'school', title: '机构名称', dataIndex: 'school_name', priority: 'primary' },
    { key: 'contact', title: '联系人', dataIndex: 'contact_name', priority: 'secondary' },
    { key: 'phone', title: '联系电话', dataIndex: 'contact_phone' },
    {
      key: 'status',
      title: '状态',
      render: (row) => <span className={styles.status}>{tenantApplicationStatusLabel(row.status)}</span>,
    },
    {
      key: 'submittedAt',
      title: '提交时间',
      render: (row) => <span className={styles.muted}>{formatDateTime(row.submitted_at)}</span>,
    },
    {
      key: 'actions',
      title: '操作',
      render: (row) => (
        <Button
          variant="primary"
          size="sm"
          icon={<Eye size={14} />}
          onClick={() => navigate(`/platform-admin/applications/${row.application_id}`)}
        >
          查看处理
        </Button>
      ),
    },
  ], [navigate])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Inbox className={styles.icon} size={28} />
            入驻申请
          </h1>
          <p className={styles.subtitle}>审核学校入驻资料并跟进入驻状态。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'loading' && <LoadingState title="正在获取入驻申请" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table
            columns={columns}
            rows={rows}
            rowKey="application_id"
            emptyTitle="暂无申请"
            emptyDescription="当前没有待查看的学校入驻申请。"
            ariaLabel="学校入驻申请列表"
          />
        </div>
      )}
    </div>
  )
}

export default ApplicationsPage
