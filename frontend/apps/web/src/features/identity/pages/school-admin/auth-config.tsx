// 认证配置页(校管侧栏,/school-admin/auth-config)。设计 §7.5:
// 两个分组「学校统一认证(CAS)」「目录服务(LDAP)」上下排列,每组右上角是启用开关和状态(未配置 / 已启用 / 已停用),
// 下方是参数表单,各自保存(启用开关也随保存生效)。
//
// CAS 与 LDAP 各一套配置,后端按 type 分别 upsert。两者的必填项不同
// (CAS 只要 https 服务器地址;LDAP 要 ldaps 地址 + 绑定账号 + 搜索基准 + 过滤式 + 匹配属性),
// 故按类型渲染各自的显式字段,不给裸 JSON 文本域。
//
// 绑定密码由后端加密保存且不回显(响应里是「已配置」占位)。修改已有配置时绑定密码留空表示不修改,
// 服务端沿用已保存的密文;首次配置必须填写(docs/01-身份与租户/03-接口设计.md)。

import { useCallback, useMemo, useState } from 'react'
import { Link } from 'react-router'
import { CircleCheck, Shield, ShieldCheck } from 'lucide-react'
import { SsoMatchField, SsoType, type SSOConfig } from '@chaimir/api-client'
import {
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  FormField,
  Input,
  PageHeader,
  PageScaffold,
  SegmentedControl,
  Skeleton,
  StickySaveBar,
  Switch,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** CAS 配置里的键(后端 validateSSOConfig 读 server_url)。 */
const CAS_SERVER_URL = 'server_url'

/** LDAP 配置里的键(后端要求这五项非空)。 */
const LDAP_FIELDS = {
  url: 'url',
  bindDn: 'bind_dn',
  bindPassword: 'bind_password',
  baseDn: 'base_dn',
  userFilter: 'user_filter',
  matchAttribute: 'match_attribute',
} as const

/** 名单匹配字段文案:决定外部账号按学工号还是手机号对应到平台账号。 */
const MATCH_FIELD_LABELS: Record<SsoMatchField, string> = {
  [SsoMatchField.NO]: '按学工号匹配',
  [SsoMatchField.PHONE]: '按手机号匹配',
}

/**
 * SchoolAdminAuthConfigPage 维护 CAS 与 LDAP 配置。
 */
export default function SchoolAdminAuthConfigPage() {
  const configs = useAsyncResource(() => api.identity.listSSOConfigs(), [], () => false)

  const byType = useMemo(() => {
    const map = new Map<SsoType, SSOConfig>()
    for (const item of configs.data ?? []) map.set(item.type, item)
    return map
  }, [configs.data])

  return (
    <PageScaffold>
      <PageHeader title="认证配置" />
      <p className="-mt-3 mb-6 text-sm text-ink-sub">
        配置只是接入参数;要让师生真正用这种方式登录,还要在{' '}
        <Link to="/school-admin/settings#settings-auth" className="font-medium text-primary underline-offset-2 hover:underline">
          学校配置
        </Link>
        里把登录方式切换过来。
      </p>

      <ResourceState
        resource={configs}
        emptyIcon={Shield}
        emptyTitle="还没有配置外部认证"
        emptyDescription="下面两种方式各配一套即可。只用平台账号密码登录的学校不需要配置。"
        skeleton={<Skeleton variant="line" lines={5} />}
      >
        {/* 表单只在权威快照就绪后挂载；重新读取时卸载，避免空初值或旧编辑态覆盖已保存配置。 */}
        {() => (
          <div className="flex flex-col gap-6">
            <CasConfigCard config={byType.get(SsoType.CAS)} onSaved={configs.reload} />
            <LdapConfigCard config={byType.get(SsoType.LDAP)} onSaved={configs.reload} />
          </div>
        )}
      </ResourceState>
    </PageScaffold>
  )
}

interface ConfigCardProps {
  config?: SSOConfig
  onSaved: () => void
}

/**
 * GroupStatus 分组右上角:已保存的状态(未配置 / 已启用 / 已停用)+ 启用开关。
 * 开关是表单的一部分,随「保存」一起提交;状态文字反映服务端已保存的值。
 */
function GroupStatus({ config, enabled, onEnabledChange, label }: { config?: SSOConfig; enabled: boolean; onEnabledChange: (value: boolean) => void; label: string }) {
  const status = !config ? '未配置' : config.enabled ? '已启用' : '已停用'
  return (
    <span className="flex items-center gap-3">
      <span className={config?.enabled ? 'text-sm font-medium text-success' : 'text-sm text-ink-sub'}>{status}</span>
      <Switch checked={enabled} aria-label={`启用${label}`} onCheckedChange={onEnabledChange} />
    </span>
  )
}

/**
 * CasConfigCard 维护 CAS 配置。
 * 服务器地址必须是公网可达的 https 地址(后端 ValidatePublicHTTPURL 会拒绝内网地址)。
 */
function CasConfigCard({ config, onSaved }: ConfigCardProps) {
  const [serverUrl, setServerUrl] = useState(readString(config?.config, CAS_SERVER_URL))
  const [matchField, setMatchField] = useState(String(config?.match_field ?? SsoMatchField.NO))
  const [enabled, setEnabled] = useState(config?.enabled ?? false)
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const { errors, validate } = useFormValidation(useCallback(() => ({
    serverUrl: isHttpsUrl(serverUrl)
      ? null
      : '统一认证服务器地址要以 https:// 开头,且必须是公网可访问的地址',
  }), [serverUrl]))

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return
      setFormError(undefined)
      setWorking(true)
      try {
        await api.identity.upsertSSOConfig({
          type: SsoType.CAS,
          config: { [CAS_SERVER_URL]: serverUrl.trim() },
          match_field: Number(matchField) as SsoMatchField,
          enabled,
        })
        toast.success('统一认证配置已保存')
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请检查服务器地址后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [enabled, matchField, onSaved, serverUrl, validate],
  )

  /** 未保存改动数:逐项与服务端配置比对(§6.5.3 第 ③ 族的保存态要「改了几处」) */
  const dirtyCount = [
    serverUrl.trim() !== readString(config?.config, CAS_SERVER_URL),
    Number(matchField) !== (config?.match_field ?? SsoMatchField.NO),
    enabled !== (config?.enabled ?? false),
  ].filter(Boolean).length

  return (
    <Card>
      <CardHeader
        title="学校统一认证(CAS)"
        description="师生跳转到学校的统一认证页登录,回跳后按名单匹配到平台账号。"
        actions={<GroupStatus config={config} enabled={enabled} onEnabledChange={setEnabled} label="学校统一认证" />}
      />
      <CardBody>
        <form onSubmit={submit} noValidate className="flex flex-col gap-4">
          <FormField
            label="统一认证服务器地址"
            htmlFor="cas-server"
            required
            helper="必须是 https 且公网可访问;内网地址会被拒绝"
            error={errors.serverUrl}
          >
            <Input
              id="cas-server"
              value={serverUrl}
              placeholder="https://sso.example.edu/cas"
              onChange={(event) => setServerUrl(event.target.value)}
              onBlur={() => validate(['serverUrl'])}
              invalid={!!errors.serverUrl}
            />
          </FormField>

          <FormField
            label="名单匹配方式"
            required
            helper="统一认证返回的账号信息按这个字段对应到平台账号"
          >
            <SegmentedControl
              aria-label="CAS 名单匹配方式"
              options={[
                { value: String(SsoMatchField.NO), label: MATCH_FIELD_LABELS[SsoMatchField.NO] },
                { value: String(SsoMatchField.PHONE), label: MATCH_FIELD_LABELS[SsoMatchField.PHONE] },
              ]}
              value={matchField}
              onValueChange={setMatchField}
            />
          </FormField>

          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          <StickySaveBar
            dirtyCount={dirtyCount}
            saving={working}
            hint="保存后可以在登录页的统一认证入口自测一次。"
            saveAction={
              <Button type="submit" variant="primary" leftIcon={CircleCheck} loading={working}>
                保存 CAS 配置
              </Button>
            }
          />
        </form>
      </CardBody>
    </Card>
  )
}

/**
 * LdapConfigCard 维护 LDAP 配置。
 * 绑定密码不回显:修改已有配置时留空表示沿用已保存的密码,填写即替换;首次配置必须填写。
 */
function LdapConfigCard({ config, onSaved }: ConfigCardProps) {
  const configured = config !== undefined
  const [url, setUrl] = useState(readString(config?.config, LDAP_FIELDS.url))
  const [bindDn, setBindDn] = useState(readString(config?.config, LDAP_FIELDS.bindDn))
  const [bindPassword, setBindPassword] = useState('')
  const [baseDn, setBaseDn] = useState(readString(config?.config, LDAP_FIELDS.baseDn))
  const [userFilter, setUserFilter] = useState(readString(config?.config, LDAP_FIELDS.userFilter))
  const [matchAttribute, setMatchAttribute] = useState(
    readString(config?.config, LDAP_FIELDS.matchAttribute),
  )
  const [matchField, setMatchField] = useState(String(config?.match_field ?? SsoMatchField.NO))
  const [enabled, setEnabled] = useState(config?.enabled ?? false)
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const { errors, validate, reset: resetValidation } = useFormValidation(useCallback(() => ({
    url: url.trim().startsWith('ldaps://')
      ? null
      : '目录服务地址要以 ldaps:// 开头 —— 明文 LDAP 不被接受',
    bindDn: bindDn.trim() === '' ? '请填写绑定账号' : null,
    bindPassword: bindPassword === '' && !configured ? '请填写绑定密码' : null,
    baseDn: baseDn.trim() === '' ? '请填写搜索基准' : null,
    userFilter: userFilter.trim() === ''
      ? '请填写用户过滤式'
      : !userFilter.includes('{username}')
        ? '请在用户过滤式中用 {username} 代表登录名'
        : null,
    matchAttribute: matchAttribute.trim() === '' ? '请填写匹配属性' : null,
  }), [baseDn, bindDn, bindPassword, configured, matchAttribute, url, userFilter]))

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return
      setFormError(undefined)
      setWorking(true)
      try {
        await api.identity.upsertSSOConfig({
          type: SsoType.LDAP,
          config: {
            [LDAP_FIELDS.url]: url.trim(),
            [LDAP_FIELDS.bindDn]: bindDn.trim(),
            [LDAP_FIELDS.bindPassword]: bindPassword,
            [LDAP_FIELDS.baseDn]: baseDn.trim(),
            [LDAP_FIELDS.userFilter]: userFilter.trim(),
            [LDAP_FIELDS.matchAttribute]: matchAttribute.trim(),
          },
          match_field: Number(matchField) as SsoMatchField,
          enabled,
        })
        toast.success('目录服务配置已保存')
        setBindPassword('')
        resetValidation()
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请检查服务器参数后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [baseDn, bindDn, bindPassword, enabled, matchAttribute, matchField, onSaved, resetValidation, url, userFilter, validate],
  )

  /**
   * 未保存改动数:逐项与服务端配置比对(§6.5.3 第 ③ 族)。
   * 绑定密码只要填了就算一处改动 —— 后端不回显它,与服务端值无从比对。
   */
  const ldapDirtyCount = [
    url.trim() !== readString(config?.config, LDAP_FIELDS.url),
    bindDn.trim() !== readString(config?.config, LDAP_FIELDS.bindDn),
    bindPassword !== '',
    baseDn.trim() !== readString(config?.config, LDAP_FIELDS.baseDn),
    userFilter.trim() !== readString(config?.config, LDAP_FIELDS.userFilter),
    matchAttribute.trim() !== readString(config?.config, LDAP_FIELDS.matchAttribute),
    Number(matchField) !== (config?.match_field ?? SsoMatchField.NO),
    enabled !== (config?.enabled ?? false),
  ].filter(Boolean).length

  return (
    <Card>
      <CardHeader
        title="目录服务(LDAP)"
        description="在平台登录页输入校内账号密码,由平台用绑定账号连接目录服务查到用户后校验密码。"
        actions={<GroupStatus config={config} enabled={enabled} onEnabledChange={setEnabled} label="目录服务" />}
      />
      <CardBody>
        <form onSubmit={submit} noValidate className="flex flex-col gap-4">
          <FormField
            label="目录服务地址"
            htmlFor="ldap-url"
            required
            helper="必须是 ldaps://(加密连接),明文 LDAP 不被接受"
            error={errors.url}
          >
            <Input
              id="ldap-url"
              value={url}
              placeholder="ldaps://ldap.example.edu:636"
              onChange={(event) => setUrl(event.target.value)}
              onBlur={() => validate(['url'])}
              invalid={!!errors.url}
            />
          </FormField>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField
              label="绑定账号"
              htmlFor="ldap-bind-dn"
              required
              helper="用于查询用户的服务账号"
              error={errors.bindDn}
            >
              <Input
                id="ldap-bind-dn"
                value={bindDn}
                placeholder="cn=svc,dc=example,dc=edu"
                onChange={(event) => setBindDn(event.target.value)}
                onBlur={() => validate(['bindDn'])}
                invalid={!!errors.bindDn}
              />
            </FormField>
            <FormField
              label="绑定密码"
              htmlFor="ldap-bind-password"
              required={!configured}
              error={errors.bindPassword}
              helper={
                configured
                  ? '已配置,不会回显;留空表示不修改,填写即替换'
                  : '保存后加密存储,不会回显'
              }
            >
              <Input
                id="ldap-bind-password"
                type="password"
                value={bindPassword}
                autoComplete="new-password"
                onChange={(event) => setBindPassword(event.target.value)}
                onBlur={() => validate(['bindPassword'])}
                invalid={!!errors.bindPassword}
              />
            </FormField>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField
              label="搜索基准"
              htmlFor="ldap-base-dn"
              required
              helper="从哪个节点开始搜索用户"
              error={errors.baseDn}
            >
              <Input
                id="ldap-base-dn"
                value={baseDn}
                placeholder="ou=people,dc=example,dc=edu"
                onChange={(event) => setBaseDn(event.target.value)}
                onBlur={() => validate(['baseDn'])}
                invalid={!!errors.baseDn}
              />
            </FormField>
            <FormField
              label="用户过滤式"
              htmlFor="ldap-filter"
              required
              helper="按什么条件定位用户,用 {username} 代表登录名"
              error={errors.userFilter}
            >
              <Input
                id="ldap-filter"
                value={userFilter}
                placeholder="(uid={username})"
                onChange={(event) => setUserFilter(event.target.value)}
                onBlur={() => validate(['userFilter'])}
                invalid={!!errors.userFilter}
              />
            </FormField>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField
              label="匹配属性"
              htmlFor="ldap-attribute"
              required
              helper="从目录里读哪个属性用于对应平台账号"
              error={errors.matchAttribute}
            >
              <Input
                id="ldap-attribute"
                value={matchAttribute}
                placeholder="employeeNumber"
                onChange={(event) => setMatchAttribute(event.target.value)}
                onBlur={() => validate(['matchAttribute'])}
                invalid={!!errors.matchAttribute}
              />
            </FormField>
            <FormField label="名单匹配方式" required helper="匹配属性的值按这个字段对应到平台账号">
              <SegmentedControl
                aria-label="LDAP 名单匹配方式"
                options={[
                  { value: String(SsoMatchField.NO), label: MATCH_FIELD_LABELS[SsoMatchField.NO] },
                  { value: String(SsoMatchField.PHONE), label: MATCH_FIELD_LABELS[SsoMatchField.PHONE] },
                ]}
                value={matchField}
                onValueChange={setMatchField}
              />
            </FormField>
          </div>


          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          <StickySaveBar
            dirtyCount={ldapDirtyCount}
            saving={working}
            hint="平台只向目录服务发起加密连接,不会把师生密码保存下来。"
            saveAction={
              <Button type="submit" variant="primary" leftIcon={ShieldCheck} loading={working}>
                保存 LDAP 配置
              </Button>
            }
          />
        </form>
      </CardBody>
    </Card>
  )
}

/** readString 从配置对象里读字符串;被脱敏的字段回空串,不把占位值当输入回填。 */
function readString(config: Record<string, unknown> | undefined, key: string): string {
  const value = config?.[key]
  return typeof value === 'string' ? value : ''
}

/** isHttpsUrl 前端先挡非 https 地址;公网可达性由后端最终判定。 */
function isHttpsUrl(value: string): boolean {
  return value.trim().startsWith('https://')
}
