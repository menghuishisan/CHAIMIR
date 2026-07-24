// 平台漏洞源页展示已接入的来源，不猜测动态配置字段。

import React from 'react'
import type { VulnSource } from '@chaimir/api-client'
import { Table, ResourceState } from '@chaimir/ui'
import { DatabaseZap } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../contest.module.css'
import { vulnLevelLabel } from '../../../../../utils/index'

/** PlatformVulnerabilitiesPage 呈现后端提供的漏洞源名称、等级、状态和最近同步信息。 */
const PlatformVulnerabilitiesPage: React.FC = () => {
  const resource = useAsyncResource(() => api.contest.listPlatformVulnSources(), [])

  if (resource.status === 'loading') {
    return <ResourceState status="loading" title="正在读取漏洞源" description="系统正在同步平台漏洞源。" />
  }
  if (resource.status === 'error') {
    return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>平台端 / 漏洞题源管理</div>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <DatabaseZap className={styles.titleIcon} size={28} />
          漏洞题源管理
        </h1>
      </div>
      <section className={styles.section}>
        <Table<VulnSource>
          rows={resource.data ?? []}
          rowKey="id"
          ariaLabel="已接入的漏洞源"
          emptyTitle="暂无漏洞源"
          emptyDescription="当前没有已接入的漏洞源。"
          columns={[
            { key: 'name', title: '名称', dataIndex: 'name', priority: 'primary' },
            { key: 'level', title: '默认等级', render: (row) => vulnLevelLabel(row.default_level) },
            { key: 'enabled', title: '状态', render: (row) => row.enabled ? '启用' : '停用' },
            { key: 'sync', title: '最近同步', render: (row) => row.last_sync_at || '尚未同步' },
          ]}
        />
      </section>
    </div>
  )
}

export default PlatformVulnerabilitiesPage
