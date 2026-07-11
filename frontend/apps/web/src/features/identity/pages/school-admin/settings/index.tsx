// AdminSettingsPage 管理当前学校租户配置，读取并更新 identity 租户配置接口。

import React, { useCallback, useMemo, useState } from 'react'
import { AuthMode } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Switch, Textarea } from '@chaimir/ui'
import { Save, Settings } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../identity-admin.module.css'
import { authModeOptions, parseJsonObject } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'


const AdminSettingsPage: React.FC = () => {
  const resource = useAsyncResource(() => api.identity.getTenantConfig(), [])
  const tenant = resource.data
  const [displayName, setDisplayName] = useState('')
  const [logoUrl, setLogoUrl] = useState('')
  const [authMode, setAuthMode] = useState(String(AuthMode.LOCAL))
  const [enableActivationCode, setEnableActivationCode] = useState(true)
  const [featureFlags, setFeatureFlags] = useState('{}')
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const effectiveDisplayName = displayName || tenant?.display_name || tenant?.name || ''
  const effectiveLogoUrl = logoUrl || tenant?.logo_url || ''
  const effectiveAuthMode = authMode || String(tenant?.auth_mode || AuthMode.LOCAL)
  const effectiveEnableActivationCode = enableActivationCode ?? Boolean(tenant?.enable_activation_code)
  const defaultFlags = useMemo(() => JSON.stringify({}, null, 2), [])
  const effectiveFeatureFlags = featureFlags === '{}' && tenant ? JSON.stringify({}, null, 2) : featureFlags || defaultFlags

  /**
   * hydrateForm 把后端租户配置填入编辑态。
   */
  const hydrateForm = useCallback(() => {
    if (!tenant) {
      return
    }
    setDisplayName(tenant.display_name || tenant.name)
    setLogoUrl(tenant.logo_url || '')
    setAuthMode(String(tenant.auth_mode))
    setEnableActivationCode(tenant.enable_activation_code)
    setFeatureFlags(JSON.stringify({}, null, 2))
  }, [tenant])

  /**
   * handleSave 提交当前租户配置。
   */
  const handleSave = useCallback(async () => {
    setSubmitting(true)
    setError(null)
    setMessage(null)
    try {
      await api.identity.updateTenantConfig({
        logo_url: effectiveLogoUrl,
        display_name: effectiveDisplayName,
        feature_flags: parseJsonObject(effectiveFeatureFlags),
        auth_mode: Number(effectiveAuthMode) as AuthMode,
        enable_activation_code: effectiveEnableActivationCode,
      })
      setMessage('学校配置已保存。')
      resource.reload()
    } catch (saveError) {
      setError(userFacingErrorMessage(saveError, '配置保存失败，请检查内容后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [effectiveAuthMode, effectiveDisplayName, effectiveEnableActivationCode, effectiveFeatureFlags, effectiveLogoUrl, resource])

  if (resource.status === 'loading') {
    return <LoadingState title="正在获取学校配置" />
  }
  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Settings size={28} />
            本校个性化与功能开关
          </h1>
          <p className={styles.subtitle}>配置当前学校展示名称、认证模式和功能开关。</p>
        </div>
        <Button variant="outline" onClick={hydrateForm}>
          使用当前配置填充
        </Button>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && (
        <Callout variant="success" title="保存成功">
          {message}
        </Callout>
      )}

      <section className={styles.panel}>
        <h2>租户配置</h2>
        <div className={styles.formGrid}>
          <label className={styles.field}>
            展示名称
            <Input fullWidth value={effectiveDisplayName} onChange={(event) => setDisplayName(event.target.value)} />
          </label>
          <label className={styles.field}>
            Logo 地址
            <Input fullWidth value={effectiveLogoUrl} onChange={(event) => setLogoUrl(event.target.value)} />
          </label>
          <label className={styles.field}>
            认证模式
            <Select fullWidth value={effectiveAuthMode} options={authModeOptions} onChange={setAuthMode} />
          </label>
          <Switch checked={effectiveEnableActivationCode} label="启用激活码开通" onChange={(event) => setEnableActivationCode(event.target.checked)} />
          <label className={styles.fieldFull}>
            功能开关
            <Textarea value={effectiveFeatureFlags} onChange={(event) => setFeatureFlags(event.target.value)} />
          </label>
        </div>
        <Button loading={submitting} icon={<Save size={16} />} onClick={handleSave}>
          保存配置
        </Button>
      </section>
    </div>
  )
}

export default AdminSettingsPage
