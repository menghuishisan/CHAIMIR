// NotificationPreferences 展示并更新当前账号的通知接收偏好。

import React, { useCallback, useState } from 'react'
import type { NotificationPreference } from '@chaimir/api-client'
import { Callout, Switch, ResourceState } from '@chaimir/ui'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { notificationPreferenceLabel } from '../../../../../utils'
import styles from '../shared.module.css'

/** NotificationPreferences 使用后端偏好列表作为开关状态权威。 */
export function NotificationPreferences(): React.ReactElement {
  const resource = useAsyncResource(() => api.notify.getPreferences(), [])
  const [updatingType, setUpdatingType] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  /** handleToggle 更新单类通知偏好并重新读取服务端状态。 */
  const handleToggle = useCallback(async (preference: NotificationPreference, enabled: boolean) => {
    setUpdatingType(preference.type)
    setMessage(null)
    setError(null)
    try {
      await api.notify.updatePreference(preference.type, { enabled })
      setMessage('通知偏好已保存。')
      resource.reload()
    } catch (updateError) {
      setError(userFacingErrorMessage(updateError, '通知偏好保存失败，请稍后重试。'))
    } finally {
      setUpdatingType(null)
    }
  }, [resource])

  if (resource.status === 'loading') return <ResourceState status="loading" title="正在获取通知偏好" />
  if (resource.status === 'error') return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />

  return (
    <section className={styles.preferencePanel} aria-labelledby="notification-preferences-title">
      <div>
        <h2 id="notification-preferences-title">通知偏好</h2>
        <p className={styles.content}>选择需要接收的站内通知类型。</p>
      </div>
      {error && <div className={styles.actionError} role="alert">{error}</div>}
      {message && <Callout variant="success" title="保存成功">{message}</Callout>}
      <div className={styles.preferenceList}>
        {(resource.data || []).map((preference) => (
          <Switch
            key={preference.type}
            checked={preference.enabled}
            disabled={updatingType === preference.type}
            label={notificationPreferenceLabel(preference.type)}
            onChange={(event) => void handleToggle(preference, event.target.checked)}
          />
        ))}
        {(resource.data || []).length === 0 && <p className={styles.content}>当前没有可配置的通知类型。</p>}
      </div>
    </section>
  )
}
