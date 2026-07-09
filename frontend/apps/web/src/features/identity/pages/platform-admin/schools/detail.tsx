// SchoolDetailPage 展示平台租户详情，数据来自 identity 平台租户接口。

import React from 'react'
import type { Tenant } from '@chaimir/api-client'
import { Button } from '@chaimir/ui'
import { Info, RefreshCw } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../../../admin/pages/dashboard.module.css'
import { authModeLabel, deployModeLabel, formatDateTime, tenantStatusLabel } from '../../../../../utils/index'


/**
 * SchoolDetailPage 读取单个租户详情。
 */
const SchoolDetailPage: React.FC = () => {
  const { id } = useParams()
  const resource = useAsyncResource<Tenant>(
    () => api.identity.getTenant(id || ''),
    [id],
    () => !id
  )
  const tenant = resource.data

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>
          <Info className={styles.icon} size={28} />
          租户详情
        </h1>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {resource.status === 'loading' && <LoadingState title="正在获取租户详情" />}
      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'empty' && (
        <EmptyState title="缺少租户编号" description="请从租户列表进入详情页。" />
      )}
      {resource.status === 'success' && tenant && (
        <>
          <div className={styles.statsGrid}>
            <div className={styles.statCard}>
              <div className={styles.statLabel}>学校名称</div>
              <div className={styles.statValue}>{tenant.display_name || tenant.name}</div>
            </div>
            <div className={styles.statCard}>
              <div className={styles.statLabel}>学校代号</div>
              <div className={styles.statValue}>{tenant.code}</div>
            </div>
            <div className={styles.statCard}>
              <div className={styles.statLabel}>租户状态</div>
              <div className={styles.statValue}>{tenantStatusLabel(tenant.status)}</div>
            </div>
            <div className={styles.statCard}>
              <div className={styles.statLabel}>到期时间</div>
              <div className={styles.statValue}>{formatDateTime(tenant.expire_at)}</div>
            </div>
          </div>

          <section className={styles.panel}>
            <h2 className={styles.panelTitle}>基础配置</h2>
            <div className={styles.metricsList}>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>部署形态</span>
                <span className={styles.metricValue}>{deployModeLabel(tenant.deploy_mode)}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>认证模式</span>
                <span className={styles.metricValue}>{authModeLabel(tenant.auth_mode)}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>激活码登录</span>
                <span className={styles.metricValue}>{tenant.enable_activation_code ? '启用' : '关闭'}</span>
              </div>
              <div className={styles.metricItem}>
                <span className={styles.metricLabel}>租户编号</span>
                <span className={styles.metricValue}>{tenant.id}</span>
              </div>
            </div>
          </section>
        </>
      )}
    </div>
  )
}

export default SchoolDetailPage
