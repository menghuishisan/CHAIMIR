// AuthConfigPage 管理当前租户 SSO 配置，敏感字段由后端脱敏和保存。

import React, { useCallback, useMemo, useState } from 'react'
import type { SSOConfig } from '@chaimir/api-client'
import { SsoMatchField, SsoType } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Select, Switch, Table } from '@chaimir/ui'
import { Link, RefreshCw, Save, Shield } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../identity-admin.module.css'
import { ssoMatchFieldLabel, ssoMatchFieldOptions, ssoTypeLabel, ssoTypeOptions } from '../../../../../utils/index'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'



const AuthConfigPage: React.FC = () => {
  const resource = useAsyncResource(() => api.identity.listSSOConfigs(), [])
  const [type, setType] = useState(String(SsoType.CAS))
  const [matchField, setMatchField] = useState(String(SsoMatchField.NO))
  const [enabled, setEnabled] = useState(true)
  const [casServerUrl, setCasServerUrl] = useState('')
  const [ldapUrl, setLdapUrl] = useState('')
  const [bindDN, setBindDN] = useState('')
  const [bindPassword, setBindPassword] = useState('')
  const [baseDN, setBaseDN] = useState('')
  const [userFilter, setUserFilter] = useState('')
  const [matchAttribute, setMatchAttribute] = useState('')
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
    setCasServerUrl(configString(item.config, 'server_url'))
    setLdapUrl(configString(item.config, 'url'))
    setBindDN(configString(item.config, 'bind_dn'))
    setBindPassword('')
    setBaseDN(configString(item.config, 'base_dn'))
    setUserFilter(configString(item.config, 'user_filter'))
    setMatchAttribute(configString(item.config, 'match_attribute'))
  }, [])

  /**
   * handleSave 调用后端 upsert 当前租户 SSO 配置。
   */
  const handleSave = useCallback(async () => {
    setError(null)
    setMessage(null)
    const selectedType = Number(type) as SsoType
    const validationError = validateSSOForm(selectedType, { casServerUrl, ldapUrl, bindDN, bindPassword, baseDN, userFilter, matchAttribute })
    if (validationError) {
      setError(validationError)
      return
    }
    setSubmitting(true)
    try {
      await api.identity.upsertSSOConfig({
        type: selectedType,
        match_field: Number(matchField) as SsoMatchField,
        enabled,
        config: buildSSOConfig(selectedType, { casServerUrl, ldapUrl, bindDN, bindPassword, baseDN, userFilter, matchAttribute }),
      })
      setMessage('认证配置已保存。')
      resource.reload()
    } catch (saveError) {
      setError(userFacingErrorMessage(saveError, '认证配置保存失败，请检查内容。'))
    } finally {
      setSubmitting(false)
    }
  }, [baseDN, bindDN, bindPassword, casServerUrl, enabled, ldapUrl, matchAttribute, matchField, resource, type, userFilter])

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
          <p className={styles.subtitle}>维护学校 CAS 或 LDAP 连接信息和本地账号匹配方式。</p>
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
          {Number(type) === SsoType.CAS ? (
            <label className={styles.field}>CAS 服务地址<Input fullWidth type="url" value={casServerUrl} placeholder="https://sso.example.edu/cas" onChange={(event) => setCasServerUrl(event.target.value)} /></label>
          ) : (
            <>
              <label className={styles.field}>目录服务地址<Input fullWidth type="url" value={ldapUrl} placeholder="ldaps://directory.example.edu:636" onChange={(event) => setLdapUrl(event.target.value)} /></label>
              <label className={styles.field}>服务账号<Input fullWidth value={bindDN} placeholder="用于查询目录的服务账号" onChange={(event) => setBindDN(event.target.value)} /></label>
              <label className={styles.field}>服务账号密码<Input fullWidth type="password" autoComplete="new-password" value={bindPassword} placeholder="保存时需要重新填写" onChange={(event) => setBindPassword(event.target.value)} /></label>
              <label className={styles.field}>用户查询范围<Input fullWidth value={baseDN} placeholder="例如 ou=people,dc=example,dc=edu" onChange={(event) => setBaseDN(event.target.value)} /></label>
              <label className={styles.field}>用户查询规则<Input fullWidth value={userFilter} placeholder="例如 (uid={username})" onChange={(event) => setUserFilter(event.target.value)} /></label>
              <label className={styles.field}>账号匹配属性<Input fullWidth value={matchAttribute} placeholder="例如 uid 或 mobile" onChange={(event) => setMatchAttribute(event.target.value)} /></label>
            </>
          )}
          <Button loading={submitting} icon={<Save size={16} />} onClick={handleSave}>
            保存配置
          </Button>
          <Callout variant="info" title="连通性说明">
            保存后，认证服务会在用户登录时验证连接；页面不会把尚未执行的连接检查显示为成功。
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

interface SSOFormValues {
  casServerUrl: string
  ldapUrl: string
  bindDN: string
  bindPassword: string
  baseDN: string
  userFilter: string
  matchAttribute: string
}

/** buildSSOConfig 把字段级表单转换为后端现行认证配置契约。 */
function buildSSOConfig(type: SsoType, values: SSOFormValues): Record<string, unknown> {
  if (type === SsoType.CAS) {
    return { server_url: values.casServerUrl.trim() }
  }
  return {
    url: values.ldapUrl.trim(),
    bind_dn: values.bindDN.trim(),
    bind_password: values.bindPassword,
    base_dn: values.baseDN.trim(),
    user_filter: values.userFilter.trim(),
    match_attribute: values.matchAttribute.trim(),
  }
}

/** validateSSOForm 在浏览器边界执行与服务端一致的必填项和安全协议校验。 */
function validateSSOForm(type: SsoType, values: SSOFormValues): string | null {
  if (type === SsoType.CAS) {
    return hasURLProtocol(values.casServerUrl, 'https:') ? null : 'CAS 服务地址必须是完整的 HTTPS 地址。'
  }
  if (!hasURLProtocol(values.ldapUrl, 'ldaps:')) {
    return '目录服务地址必须是完整的 LDAPS 地址。'
  }
  if (![values.bindDN, values.bindPassword, values.baseDN, values.userFilter, values.matchAttribute].every((value) => value.trim())) {
    return '请完整填写服务账号、密码、用户查询范围、查询规则和账号匹配属性。'
  }
  return null
}

/** hasURLProtocol 校验连接地址具有指定安全协议和有效主机名。 */
function hasURLProtocol(raw: string, protocol: 'https:' | 'ldaps:'): boolean {
  try {
    const parsed = new URL(raw.trim())
    return parsed.protocol === protocol && Boolean(parsed.hostname)
  } catch {
    return false
  }
}

/** configString 从后端脱敏配置中读取可展示字符串字段。 */
function configString(config: Record<string, unknown>, key: string): string {
  const value = config[key]
  return typeof value === 'string' && value !== '******' ? value : ''
}

export default AuthConfigPage
