// SchoolDetailPage 展示平台租户详情，数据来自 identity 平台租户接口。

import React, { useEffect, useState } from 'react'
import type { Tenant } from '@chaimir/api-client'
import { TenantStatus } from '@chaimir/api-client'
import { Button, Callout, Input, Select, ResourceState } from '@chaimir/ui'
import { Info, RefreshCw, Save } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource, usePendingAction } from '../../../../../hooks'
import styles from '../../../../admin/pages/dashboard.module.css'
import { authModeLabel, deployModeLabel, formatDateTime, tenantStatusLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'


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
  const [status, setStatus] = useState(String(TenantStatus.ACTIVE))
  const [expireAt, setExpireAt] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const { pendingAction, runPendingAction } = usePendingAction()

  useEffect(() => {
    if (!tenant) return
    setStatus(String(tenant.status))
    setExpireAt(tenant.expire_at?.slice(0, 10) || '')
  }, [tenant])

  /** updateTenant 保存平台控制的租户状态和到期时间。 */
  const updateTenant = async () => {
    if (!id) return
    setError('')
    try {
      await api.identity.updateTenant(id, { status: Number(status) as TenantStatus, expire_at: expireAt || undefined })
      setMessage('租户状态已更新。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '租户状态更新失败，请稍后重试。'))
    }
  }

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

      {message && <Callout variant="success" title="操作完成">{message}</Callout>}
      {error && <Callout variant="danger" title="操作未完成">{error}</Callout>}

      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取租户详情" />}
      {resource.status === 'error' && (
        <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'empty' && (
        <ResourceState status="empty" title="缺少租户编号" description="请从租户列表进入详情页。" />
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
            </div>
          </section>
          <section className={styles.panel}>
            <h2 className={styles.panelTitle}>状态维护</h2>
            <div className={styles.metricsList}>
              <Select value={status} onChange={setStatus} options={[{ value: '1', label: '正常' }, { value: '2', label: '停用' }, { value: '3', label: '已到期' }]} />
              <Input type="date" value={expireAt} onChange={(event) => setExpireAt(event.target.value)} />
              <Button icon={<Save size={15} />} loading={pendingAction === 'tenant'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('tenant', updateTenant)}>保存状态</Button>
            </div>
          </section>
        </>
      )}
    </div>
  )
}

export default SchoolDetailPage
