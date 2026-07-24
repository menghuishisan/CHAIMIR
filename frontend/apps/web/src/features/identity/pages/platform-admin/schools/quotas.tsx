// QuotasPage 为指定租户提交沙箱配额，复用 sandbox 后端配额接口。

import React, { useCallback, useEffect, useState } from 'react'
import type { SandboxQuota, SandboxQuotaRequest } from '@chaimir/api-client'
import { Button, Callout, FormField, Input, ResourceState } from '@chaimir/ui'
import { PieChart, Save } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../../../admin/pages/list.module.css'
import { sandboxQuotaFieldLabels } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'

type QuotaForm = Record<keyof Omit<SandboxQuota, 'tenant_id' | 'active_sandbox_count'>, string>

const initialForm: QuotaForm = {
  max_concurrent_sandbox: '',
  max_cpu: '',
  max_memory_mb: '',
  idle_timeout_min: '',
  max_lifetime_min: '',
  max_keepalive_min: '',
  max_snapshot_retention_min: '',
}

/** quotaToForm 把后端权威配额转换为可编辑表单。 */
function quotaToForm(quota: SandboxQuota): QuotaForm {
  return {
    max_concurrent_sandbox: String(quota.max_concurrent_sandbox),
    max_cpu: String(quota.max_cpu),
    max_memory_mb: String(quota.max_memory_mb),
    idle_timeout_min: String(quota.idle_timeout_min),
    max_lifetime_min: String(quota.max_lifetime_min),
    max_keepalive_min: String(quota.max_keepalive_min),
    max_snapshot_retention_min: String(quota.max_snapshot_retention_min),
  }
}

/**
 * toQuotaRequest 把表单字符串转换为后端配额请求。
 */
function toQuotaRequest(tenantId: string, form: QuotaForm): SandboxQuotaRequest {
  return {
    tenant_id: tenantId,
    max_concurrent_sandbox: Number(form.max_concurrent_sandbox),
    max_cpu: Number(form.max_cpu),
    max_memory_mb: Number(form.max_memory_mb),
    idle_timeout_min: Number(form.idle_timeout_min),
    max_lifetime_min: Number(form.max_lifetime_min),
    max_keepalive_min: Number(form.max_keepalive_min),
    max_snapshot_retention_min: Number(form.max_snapshot_retention_min),
  }
}

/**
 * QuotasPage 提交指定租户的完整沙箱配额。
 */
const QuotasPage: React.FC = () => {
  const { id } = useParams()
  const [form, setForm] = useState<QuotaForm>(initialForm)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(async () => {
    if (!id) throw new Error('缺少租户编号，请从租户列表进入配额页。')
    return api.sandbox.getQuota(id)
  }, [id])

  useEffect(() => {
    if (resource.data) setForm(quotaToForm(resource.data))
  }, [resource.data])

  const handleSubmit = useCallback(async () => {
    if (!id) {
      setError('缺少租户编号，请从租户列表进入配额页')
      return
    }
    setSaving(true)
    setError(null)
    setMessage(null)
    try {
      const saved = await api.sandbox.updateQuota(toQuotaRequest(id, form))
      setForm(quotaToForm(saved))
      setMessage('资源配额已更新')
    } catch (submitError) {
      setError(userFacingErrorMessage(submitError, '资源配额提交失败，请稍后重试。'))
    } finally {
      setSaving(false)
    }
  }, [form, id])

  if (resource.status === 'loading') {
    return <ResourceState status="loading" title="正在读取资源配额" description="系统正在同步这所学校的当前资源上限。" />
  }

  if (resource.status === 'error') {
    return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <PieChart className={styles.icon} size={28} />
            资源配额管控
          </h1>
          <p className={styles.subtitle}>为当前学校设置沙箱并发、算力和保留时长上限。</p>
        </div>
      </div>

      {message && <Callout variant="success" title="配额已更新">{message}</Callout>}
      {error && <Callout variant="danger" title="配额未更新">{error}</Callout>}

      <div className={styles.tableWrap}>
        <div className={styles.formGrid}>
          {(Object.keys(sandboxQuotaFieldLabels) as Array<keyof QuotaForm>).map((field) => (
            <FormField key={field} label={sandboxQuotaFieldLabels[field]}>
              <Input
                fullWidth
                min={0}
                type="number"
                value={form[field]}
                onChange={(event) => setForm((current) => ({ ...current, [field]: event.target.value }))}
              />
            </FormField>
          ))}
          <Button icon={<Save size={16} />} loading={saving} onClick={handleSubmit}>
            提交变更
          </Button>
        </div>
      </div>
    </div>
  )
}

export default QuotasPage
