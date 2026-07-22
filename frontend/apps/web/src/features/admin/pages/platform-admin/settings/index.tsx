// SettingsPage 管理平台全局配置，读取并按版本更新 admin 配置接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { ConfigChangeLog, SystemConfig } from '@chaimir/api-client'
import { AdminScope } from '@chaimir/api-client'
import { Button, Callout, Modal, Switch, Table, Textarea } from '@chaimir/ui'
import { History, RefreshCw, RotateCcw, Save, Settings } from 'lucide-react'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../list.module.css'
import formStyles from './settings.module.css'
import { parseJsonObject, stringifyJsonObject, systemConfigLabel } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

const SettingsPage: React.FC = () => {
  const resource = useAsyncResource(() => api.admin.listConfigs({ scope: AdminScope.GLOBAL }), [])
  const [editing, setEditing] = useState<Record<string, string>>({})
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
   * valueFor 返回当前编辑态内容，首次编辑时使用后端值。
   */
  const valueFor = useCallback((config: SystemConfig): string => (
    editing[config.key] ?? stringifyJsonObject(config.value)
  ), [editing])

  /**
   * handleSave 按配置版本更新后端配置。
   */
  const handleSave = useCallback(async (config: SystemConfig, nextValue?: Record<string, unknown>) => {
    setSubmittingKey(config.key)
    setError(null)
    setMessage(null)
    try {
      const value: Record<string, unknown> = nextValue ?? parseJsonObject<Record<string, unknown>>(valueFor(config))
      await api.admin.updateConfig(config.key, {
        scope: config.scope,
        tenant_id: config.tenant_id,
        value,
        version: config.version,
      })
      setMessage(`${systemConfigLabel(config.key)}已保存。`)
      setEditing((current) => {
        const next = { ...current }
        delete next[config.key]
        return next
      })
      resource.reload()
    } catch (saveError) {
      setError(userFacingErrorMessage(saveError, '配置保存失败，请检查内容后重试。'))
    } finally {
      setSubmittingKey(null)
    }
  }, [resource, valueFor])

  /**
   * handleMaintenanceToggle 通过同一配置接口切换维护模式。
   */
  const handleMaintenanceToggle = useCallback((enabled: boolean) => {
    if (!maintenanceConfig) {
      setError('维护模式配置尚未就绪，暂时无法切换。')
      return
    }
    handleSave(maintenanceConfig, { ...maintenanceConfig.value, enabled })
  }, [handleSave, maintenanceConfig])

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
    if (!historyConfig || !window.confirm('确定回退到这次变更之前吗？')) return
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

      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取系统配置" />}
      {resource.status === 'empty' && <EmptyState title="暂无配置" description="当前没有可编辑的平台全局参数。" />}
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

          {configs.map((config) => (
            <section className={formStyles.panel} key={config.id}>
              <header className={formStyles.panelHeader}>
                <div>
                  <h2>{systemConfigLabel(config.key)}</h2>
                  <p>当前版本 {config.version}</p>
                </div>
                <span className={styles.status}>{config.scope === AdminScope.GLOBAL ? '平台' : '租户'}</span>
              </header>
              <Textarea
                value={valueFor(config)}
                resize="vertical"
                onChange={(event) => setEditing((current) => ({ ...current, [config.key]: event.target.value }))}
              />
              <Button
                icon={<Save size={16} />}
                loading={submittingKey === config.key}
                onClick={() => handleSave(config)}
              >
                保存配置
              </Button>
              <Button variant="outline" icon={<History size={16} />} onClick={() => void openHistory(config)}>变更历史</Button>
            </section>
          ))}
        </div>
      )}
      <Modal open={historyConfig !== null} title={historyConfig ? `${systemConfigLabel(historyConfig.key)}变更历史` : '配置变更历史'} size="lg" onClose={() => setHistoryConfig(null)}>
        {loadingHistory ? <LoadingState title="正在获取变更历史" /> : (
          <Table
            rows={history}
            rowKey="id"
            ariaLabel="配置变更历史"
            emptyTitle="暂无变更历史"
            emptyDescription="该配置还没有可回退的变更记录。"
            columns={[
              { key: 'time', title: '变更时间', dataIndex: 'created_at', priority: 'primary' },
              { key: 'operator', title: '操作人', dataIndex: 'operator_id' },
              { key: 'action', title: '操作', render: (row: ConfigChangeLog) => <Button variant="outline" size="sm" icon={<RotateCcw size={14} />} onClick={() => void rollbackConfig(row)}>回退到此前</Button> },
            ]}
          />
        )}
      </Modal>
    </div>
  )
}

export default SettingsPage
