// 认证配置页(校管侧栏,/school-admin/auth-config)。
//
// CAS 与 LDAP 各一套配置,后端按 type 分别 upsert。两者的必填项不同
// (CAS 只要 https 服务器地址;LDAP 要 ldaps 地址 + 绑定账号 + 搜索基准 + 过滤式 + 匹配属性),
// 故按类型渲染各自的显式字段,不给裸 JSON 文本域。
//
// 绑定密码由后端加密保存且不回显(响应里是「已配置」占位)。编辑时留空即报错 ——
// 后端 secureSSOConfig 明确拒绝把占位值当密码,故这里要求每次保存都重填密码,并说明原因。

import { useCallback, useMemo, useState } from 'react'
import { CircleCheck, Shield, ShieldCheck } from 'lucide-react'
import { SsoMatchField, SsoType, type SSOConfig } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  Checkbox,
  DescriptionList,
  FormField,
  Input,
  PageHeader,
  PageScaffold,
  PageSection,
  SegmentedControl,
  Skeleton,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
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
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '系统配置' }, { label: '认证配置' }]} />}
        title="认证配置"
        description="接入学校统一认证或目录服务。配置好并启用后,师生可以用校内账号登录。"
        icon={Shield}
      />

      <ResourceState
        resource={configs}
        emptyIcon={Shield}
        emptyTitle="还没有配置外部认证"
        emptyDescription="下面两种方式各配一套即可。只用平台账号密码登录的学校不需要配置。"
        skeleton={<Skeleton variant="line" lines={5} />}
      >
        {() => null}
      </ResourceState>

      <PageSection
        title="学校统一认证(CAS)"
        description="师生跳转到学校的统一认证页登录,回跳后按名单匹配到平台账号。"
      >
        <CasConfigCard config={byType.get(SsoType.CAS)} onSaved={configs.reload} />
      </PageSection>

      <PageSection
        title="目录服务(LDAP)"
        description="在平台登录页输入校内账号密码,由平台向目录服务校验。"
      >
        <LdapConfigCard config={byType.get(SsoType.LDAP)} onSaved={configs.reload} />
      </PageSection>

      <Callout tone="info">
        配置只是接入参数。要让师生真正用这种方式登录,还要在租户配置页把登录方式切换过来。
      </Callout>
    </PageScaffold>
  )
}

interface ConfigCardProps {
  config?: SSOConfig
  onSaved: () => void
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

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!isHttpsUrl(serverUrl)) {
        setFormError('统一认证服务器地址要以 https:// 开头,且必须是公网可访问的地址')
        return
      }
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
    [enabled, matchField, onSaved, serverUrl],
  )

  return (
    <Card>
      <CardHeader
        title="CAS 参数"
        description="填写学校统一认证服务的根地址,例如 https://sso.example.edu/cas。"
        actions={
          config?.enabled ? <Badge tone="success">已启用</Badge> : <Badge tone="neutral">未启用</Badge>
        }
      />
      <CardBody>
        <form onSubmit={submit} noValidate className="flex flex-col gap-4">
          <FormField
            label="统一认证服务器地址"
            htmlFor="cas-server"
            required
            helper="必须是 https 且公网可访问;内网地址会被拒绝"
          >
            <Input
              id="cas-server"
              value={serverUrl}
              placeholder="https://sso.example.edu/cas"
              onChange={(event) => setServerUrl(event.target.value)}
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

          <Checkbox
            checked={enabled}
            label="启用这套配置"
            onCheckedChange={(checked) => setEnabled(checked === true)}
          />

          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          <div className="flex items-center gap-2">
            <Button type="submit" variant="primary" leftIcon={CircleCheck} loading={working}>
              保存 CAS 配置
            </Button>
            <span className="text-sm text-ink-sub">
              保存后可以在登录页的统一认证入口自测一次。
            </span>
          </div>
        </form>
      </CardBody>
    </Card>
  )
}

/**
 * LdapConfigCard 维护 LDAP 配置。
 * 绑定密码不回显:后端保存后返回占位值,直接提交占位值会被拒绝,
 * 故每次保存都要求重填 —— 界面说明原因,不让管理员以为是 bug。
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

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!url.trim().startsWith('ldaps://')) {
        setFormError('目录服务地址要以 ldaps:// 开头 —— 明文 LDAP 不被接受')
        return
      }
      const missing = [
        bindDn.trim() === '' ? '绑定账号' : '',
        bindPassword === '' ? '绑定密码' : '',
        baseDn.trim() === '' ? '搜索基准' : '',
        userFilter.trim() === '' ? '用户过滤式' : '',
        matchAttribute.trim() === '' ? '匹配属性' : '',
      ].filter(Boolean)
      if (missing.length > 0) {
        setFormError(`还需要填写:${missing.join('、')}`)
        return
      }
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
        onSaved()
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '保存没有成功,请检查服务器参数后重试。'))
      } finally {
        setWorking(false)
      }
    },
    [baseDn, bindDn, bindPassword, enabled, matchAttribute, matchField, onSaved, url, userFilter],
  )

  return (
    <Card>
      <CardHeader
        title="LDAP 参数"
        description="平台用绑定账号连接目录服务,按过滤式查到用户后再校验其密码。"
        actions={
          config?.enabled ? <Badge tone="success">已启用</Badge> : <Badge tone="neutral">未启用</Badge>
        }
      />
      <CardBody>
        <form onSubmit={submit} noValidate className="flex flex-col gap-4">
          <FormField
            label="目录服务地址"
            htmlFor="ldap-url"
            required
            helper="必须是 ldaps://(加密连接),明文 LDAP 不被接受"
          >
            <Input
              id="ldap-url"
              value={url}
              placeholder="ldaps://ldap.example.edu:636"
              onChange={(event) => setUrl(event.target.value)}
            />
          </FormField>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField
              label="绑定账号"
              htmlFor="ldap-bind-dn"
              required
              helper="用于查询用户的服务账号"
            >
              <Input
                id="ldap-bind-dn"
                value={bindDn}
                placeholder="cn=svc,dc=example,dc=edu"
                onChange={(event) => setBindDn(event.target.value)}
              />
            </FormField>
            <FormField
              label="绑定密码"
              htmlFor="ldap-bind-password"
              required
              helper={
                configured
                  ? '已保存的密码不会回显,每次保存都需要重新填写'
                  : '保存后加密存储,不会回显'
              }
            >
              <Input
                id="ldap-bind-password"
                type="password"
                value={bindPassword}
                autoComplete="new-password"
                onChange={(event) => setBindPassword(event.target.value)}
              />
            </FormField>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField
              label="搜索基准"
              htmlFor="ldap-base-dn"
              required
              helper="从哪个节点开始搜索用户"
            >
              <Input
                id="ldap-base-dn"
                value={baseDn}
                placeholder="ou=people,dc=example,dc=edu"
                onChange={(event) => setBaseDn(event.target.value)}
              />
            </FormField>
            <FormField
              label="用户过滤式"
              htmlFor="ldap-filter"
              required
              helper="按什么条件定位用户,用 %s 代表登录名"
            >
              <Input
                id="ldap-filter"
                value={userFilter}
                placeholder="(uid=%s)"
                onChange={(event) => setUserFilter(event.target.value)}
              />
            </FormField>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <FormField
              label="匹配属性"
              htmlFor="ldap-attribute"
              required
              helper="从目录里读哪个属性用于对应平台账号"
            >
              <Input
                id="ldap-attribute"
                value={matchAttribute}
                placeholder="employeeNumber"
                onChange={(event) => setMatchAttribute(event.target.value)}
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

          <Checkbox
            checked={enabled}
            label="启用这套配置"
            onCheckedChange={(checked) => setEnabled(checked === true)}
          />

          {configured ? (
            <DescriptionList
              dense
              items={[
                {
                  term: '当前状态',
                  description: config.enabled ? '已启用,师生可用校内账号登录' : '已配置但未启用',
                },
                {
                  term: '匹配方式',
                  description: MATCH_FIELD_LABELS[config.match_field],
                },
              ]}
            />
          ) : null}

          {formError ? <Callout tone="danger">{formError}</Callout> : null}

          <div className="flex items-center gap-2">
            <Button type="submit" variant="primary" leftIcon={ShieldCheck} loading={working}>
              保存 LDAP 配置
            </Button>
            <span className="text-sm text-ink-sub">
              平台只向目录服务发起加密连接,不会把师生密码保存下来。
            </span>
          </div>
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
