// SettingsPage 管理平台全局配置，读取并按版本更新 admin 配置接口。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError, SystemConfig } from '@chaimir/api-client'
import { AdminScope } from '@chaimir/api-client'
import { Button, Callout, Switch, Textarea } from '@chaimir/ui'
import { RefreshCw, Save, Settings } from 'lucide-react'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../list.module.css'
import formStyles from './settings.module.css'
import { parseJsonObject, stringifyJsonObject, systemConfigLabel } from '../../../../../utils/index'

const SettingsPage: React.FC = () => {
  const resource = useAsyncResource(() => api.admin.listConfigs({ scope: AdminScope.GLOBAL }), [])
  const [editing, setEditing] = useState<Record<string, string>>({})
  const [submittingKey, setSubmittingKey] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

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
      setError((saveError as ApiError).message || (saveError as Error).message || '配置保存失败，请检查内容后重试。')
    } finally {
      setSubmittingKey(null)
    }
  }, [resource, valueFor])

  /**
   * handleMaintenanceToggle 通过同一配置接口切换维护模式。
   */
  const handleMaintenanceToggle = useCallback((enabled: boolean) => {
    if (!maintenanceConfig) {
      setError('后端尚未返回维护模式配置，无法切换。')
      return
    }
    handleSave(maintenanceConfig, { ...maintenanceConfig.value, enabled })
  }, [handleSave, maintenanceConfig])

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Settings className={styles.icon} size={28} />
            平台全局参数
          </h1>
          <p className={styles.subtitle}>读取后端系统配置并按版本保存，敏感值由后端负责脱敏。</p>
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
            <p>开启后，平台会按后端策略限制普通用户访问。</p>
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
            </section>
          ))}
        </div>
      )}
    </div>
  )
}

export default SettingsPage
