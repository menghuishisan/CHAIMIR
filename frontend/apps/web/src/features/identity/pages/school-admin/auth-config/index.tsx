// AuthConfigPage 管理当前租户 SSO 配置，敏感字段由后端脱敏和保存。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError, SSOConfig } from '@chaimir/api-client'
import { SsoMatchField, SsoType } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Select, Switch, Table, Textarea } from '@chaimir/ui'
import { Link, RefreshCw, Save, Shield } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../identity-admin.module.css'
import { parseJsonObject, ssoMatchFieldLabel, ssoMatchFieldOptions, ssoTypeLabel, ssoTypeOptions } from '../../../../../utils/index'



const AuthConfigPage: React.FC = () => {
  const resource = useAsyncResource(() => api.identity.listSSOConfigs(), [])
  const [type, setType] = useState(String(SsoType.CAS))
  const [matchField, setMatchField] = useState(String(SsoMatchField.NO))
  const [enabled, setEnabled] = useState(true)
  const [config, setConfig] = useState('{\n  "login_url": "",\n  "service_url": ""\n}')
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  /**
   * fillConfig 把后端返回配置填入编辑表单。
   */
  const fillConfig = useCallback((item: SSOConfig) => {
    setType(String(item.type))
    setMatchField(String(item.match_field))
    setEnabled(item.enabled)
    setConfig(JSON.stringify(item.config, null, 2))
  }, [])

  /**
   * handleSave 调用后端 upsert 当前租户 SSO 配置。
   */
  const handleSave = useCallback(async () => {
    setSubmitting(true)
    setError(null)
    setMessage(null)
    try {
      await api.identity.upsertSSOConfig({
        type: Number(type) as SsoType,
        match_field: Number(matchField) as SsoMatchField,
        enabled,
        config: parseJsonObject(config),
      })
      setMessage('认证配置已保存。')
      resource.reload()
    } catch (saveError) {
      setError((saveError as ApiError).message || (saveError as Error).message || '认证配置保存失败，请检查内容。')
    } finally {
      setSubmitting(false)
    }
  }, [config, enabled, matchField, resource, type])

  const columns = useMemo<TableColumn<SSOConfig>[]>(() => [
    { key: 'type', title: '类型', render: (row) => ssoTypeLabel(row.type), priority: 'primary' },
    { key: 'match', title: '匹配字段', render: (row) => (ssoMatchFieldLabel(row.match_field)) },
    { key: 'enabled', title: '状态', render: (row) => (row.enabled ? '已启用' : '已停用') },
    {
      key: 'action',
      title: '操作',
      render: (row) => (
        <Button variant="ghost" size="sm" onClick={() => fillConfig(row)}>
          编辑
        </Button>
      ),
    },
  ], [fillConfig])

  const rows = resource.data || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}>
            <Shield size={28} />
            统一认证对接
          </h1>
          <p className={styles.subtitle}>维护学校 CAS 或 LDAP 配置，配置字段按后端 JSON 契约保存。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && (
        <Callout variant="success" title="保存成功">
          {message}
        </Callout>
      )}

      <div className={styles.grid}>
        <section className={styles.panel}>
          <h2>认证配置</h2>
          <label className={styles.field}>
            认证类型
            <Select fullWidth value={type} options={ssoTypeOptions} onChange={setType} />
          </label>
          <label className={styles.field}>
            匹配字段
            <Select fullWidth value={matchField} options={ssoMatchFieldOptions} onChange={setMatchField} />
          </label>
          <Switch checked={enabled} label={enabled ? '已启用' : '已停用'} onChange={(event) => setEnabled(event.target.checked)} />
          <label className={styles.field}>
            配置 JSON
            <Textarea value={config} onChange={(event) => setConfig(event.target.value)} />
          </label>
          <Button loading={submitting} icon={<Save size={16} />} onClick={handleSave}>
            保存配置
          </Button>
          <Callout variant="info" title="连通性说明">
            当前前端只保存配置；连通性由后端认证流程和运维监控确认，不在页面伪造测试结果。
          </Callout>
        </section>

        <section className={styles.panel}>
          <h2>
            <Link size={18} />
            已配置认证
          </h2>
          {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
          {resource.status === 'loading' && <LoadingState title="正在获取认证配置" />}
          {(resource.status === 'success' || resource.status === 'empty') && (
            <Table
              columns={columns}
              rows={rows}
              rowKey="id"
              emptyTitle="暂无认证配置"
              emptyDescription="当前学校尚未配置统一认证。"
              ariaLabel="统一认证配置列表"
            />
          )}
        </section>
      </div>
    </div>
  )
}

export default AuthConfigPage
