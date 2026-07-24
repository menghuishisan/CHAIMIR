// SettingsPage 管理平台全局配置，读取并按版本更新 admin 配置接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { ConfigChangeLog, SystemConfig } from '@chaimir/api-client'
import { AdminScope } from '@chaimir/api-client'
import { Button, Callout, DescriptionList, Modal, Switch, Table, useConfirm, ResourceState } from '@chaimir/ui'
import { History, RefreshCw, RotateCcw, Settings } from 'lucide-react'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../list.module.css'
import formStyles from './settings.module.css'
import { formatDateTime, systemConfigLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const SettingsPage: React.FC = () => {
  const confirm = useConfirm()
  const resource = useAsyncResource(() => api.admin.listConfigs({ scope: AdminScope.GLOBAL }), [])
  const [submittingKey, setSubmittingKey] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [historyConfig, setHistoryConfig] = useState<SystemConfig | null>(null)
  const [history, setHistory] = useState<ConfigChangeLog[]>([])
  const [loadingHistory, setLoadingHistory] = useState(false)

  const configs = useMemo(() => resource.data || [], [resource.data])
  const maintenanceConfig = useMemo(
    () => configs.find((config) => config.key === 'maintenance_mode'),
    [configs],
  )
  const maintenanceEnabled = maintenanceConfig?.value?.enabled === true

  /**
   * handleMaintenanceToggle 通过同一配置接口切换维护模式。
   */
  const handleMaintenanceToggle = useCallback(async (enabled: boolean) => {
    if (!maintenanceConfig) {
      setError('维护模式配置尚未就绪，暂时无法切换。')
      return
    }
    setSubmittingKey(maintenanceConfig.key)
    setError(null)
    setMessage(null)
    try {
      await api.admin.updateConfig(maintenanceConfig.key, {
        scope: maintenanceConfig.scope,
        tenant_id: maintenanceConfig.tenant_id,
        value: { ...maintenanceConfig.value, enabled },
        version: maintenanceConfig.version,
      })
      setMessage('维护模式已更新。')
      resource.reload()
    } catch (saveError) {
      setError(userFacingErrorMessage(saveError, '维护模式更新失败，请稍后重试。'))
    } finally {
      setSubmittingKey(null)
    }
  }, [maintenanceConfig, resource])

  /** openHistory 读取指定配置的服务端变更记录。 */
  const openHistory = async (config: SystemConfig) => {
    setHistoryConfig(config)
    setLoadingHistory(true)
    setError(null)
    try {
      const response = await api.admin.listConfigHistory(config.key, { scope: config.scope, tenant_id: config.tenant_id, page: 1, size: 50 })
      setHistory(response.list)
    } catch (historyError) {
      setError(userFacingErrorMessage(historyError, '配置历史读取失败，请稍后重试。'))
      setHistory([])
    } finally {
      setLoadingHistory(false)
    }
  }

  /** rollbackConfig 按当前版本回退到所选历史记录的变更前值。 */
  const rollbackConfig = async (log: ConfigChangeLog) => {
    if (!historyConfig) return
    const confirmed = await confirm({ title: '回退配置', description: '回退会创建一条新的配置变更记录，并恢复到所选变更之前的值。', confirmLabel: '确认回退' })
    if (!confirmed) return
    setSubmittingKey(historyConfig.key)
    setError(null)
    try {
      await api.admin.rollbackConfig(historyConfig.key, {
        scope: historyConfig.scope,
        tenant_id: historyConfig.tenant_id,
        version: historyConfig.version,
        change_log_id: log.id,
      })
      setMessage(`${systemConfigLabel(historyConfig.key)}已回退。`)
      setHistoryConfig(null)
      resource.reload()
    } catch (rollbackError) {
      setError(userFacingErrorMessage(rollbackError, '配置回退失败，请刷新后重试。'))
    } finally {
      setSubmittingKey(null)
    }
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Settings className={styles.icon} size={28} />
            平台全局参数
          </h1>
          <p className={styles.subtitle}>维护平台全局参数并查看配置变更记录。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {error && <div className={formStyles.error}>{error}</div>}
      {message && (
        <Callout variant="success" title="配置已更新">
          {message}
        </Callout>
      )}

      {resource.status === 'error' && <ResourceState status="error" error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <ResourceState status="loading" title="正在获取系统配置" />}
      {resource.status === 'empty' && <ResourceState status="empty" title="暂无配置" description="当前没有可编辑的平台全局参数。" />}
      {resource.status === 'success' && (
        <div className={formStyles.grid}>
          <section className={formStyles.panel}>
            <h2>维护模式</h2>
            <p>开启后，普通用户将暂时无法访问平台功能。</p>
            <Switch
              checked={maintenanceEnabled}
              label={maintenanceEnabled ? '已开启' : '已关闭'}
              disabled={!maintenanceConfig || submittingKey === maintenanceConfig.key}
              onChange={(event) => handleMaintenanceToggle(event.target.checked)}
            />
          </section>

          {configs.filter((config) => config.key !== 'maintenance_mode').map((config) => (
            <section className={formStyles.panel} key={config.id}>
              <header className={formStyles.panelHeader}>
                <div>
                  <h2>{systemConfigLabel(config.key)}</h2>
                  <p>当前版本 {config.version}</p>
                </div>
                <span className={styles.status}>{config.scope === AdminScope.GLOBAL ? '平台' : '租户'}</span>
              </header>
              <DescriptionList items={[
                { key: 'version', label: '当前版本', value: config.version },
                { key: 'updated', label: '最近更新', value: formatDateTime(config.updated_at) },
              ]} />
              <Button variant="outline" icon={<History size={16} />} onClick={() => void openHistory(config)}>变更历史</Button>
            </section>
          ))}
        </div>
      )}
      <Modal open={historyConfig !== null} title={historyConfig ? `${systemConfigLabel(historyConfig.key)}变更历史` : '配置变更历史'} size="lg" onClose={() => setHistoryConfig(null)}>
        {loadingHistory ? <ResourceState status="loading" title="正在获取变更历史" /> : (
          <Table
            rows={history}
            rowKey="id"
            ariaLabel="配置变更历史"
            emptyTitle="暂无变更历史"
            emptyDescription="该配置还没有可回退的变更记录。"
            columns={[
              { key: 'time', title: '变更时间', dataIndex: 'created_at', priority: 'primary' },
              { key: 'action', title: '操作', render: (row: ConfigChangeLog) => <Button variant="outline" size="sm" icon={<RotateCcw size={14} />} onClick={() => void rollbackConfig(row)}>回退到此前</Button> },
            ]}
          />
        )}
      </Modal>
    </div>
  )
}

export default SettingsPage
