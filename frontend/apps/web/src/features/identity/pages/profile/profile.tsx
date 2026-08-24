// 个人中心页(共享入口,{prefix}/profile,顶栏头像菜单进入)。
//
// 能力按身份分化(后端 api_me.go:四条路由都只挂 authn.Middleware(),分叉在 service 层):
//   租户账号(学生/教师/校管)= 资料 + 改密 + 换绑手机号 + 登录会话
//   平台账号               = 资料只读 + 改密 + 登录会话(ChangeMyPhone 拒平台身份,
//                            platform_admin 表也没有手机号/学工号/职称/租户列)
// 是否渲染换绑手机号由 props 显式声明,页面不在运行时判角色枚举 —— 与铃铛归属同一处理。

import { useCallback, useId, useMemo, useState, type ReactNode } from 'react'
import { KeyRound, LogOut, ShieldCheck, Smartphone } from 'lucide-react'
import { SessionStatus, SmsScene, TenantStatus, type Session } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  DataPanel,
  FormField,
  Input,
  ObjectIdentity,
  PageBody,
  PageScaffold,
  PageSection,
  StatusIndicator,
  Table,
  toast,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { invalidateAppResource } from '../../../../app/resourceInvalidation'
import { ResourceState } from '../../../../components/ResourceState'
import { useSession } from '../../../../components/RoleGuard'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import {
  accountStatusLabel,
  baseIdentityLabel,
  sessionStatusLabel,
  tenantStatusLabel,
  userRolesLabel,
} from '../../../../utils/labels/identity'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { accountStatusTone, sessionStatusTone, tenantStatusTone } from '../../statusPresentation'
import {
  confirmPasswordError,
  passwordRuleError,
  phoneError,
  requiredError,
  useFieldErrors,
  useSmsCooldown,
} from '../../authForm'

export interface ProfilePageProps {
  /**
   * 是否提供换绑手机号。
   * 平台账号传 false:后端 ChangeMyPhone 走 requireTenantSession,平台身份返回禁止,
   * 且 platform_admin 表无手机号列 —— 渲染这个表单等于给一个必然失败的入口。
   */
  canChangePhone: boolean
  /** 平台管理员的 status 使用 platform_admin 状态枚举,不能按租户账号状态解释。 */
  statusKind?: 'account' | 'platform'
}

/**
 * ProfilePage 呈现账号资料并承载改密、换绑手机号与会话管理。
 */
export default function ProfilePage({ canChangePhone, statusKind = 'account' }: ProfilePageProps) {
  const { me, reload } = useSession()
  const account = me.account
  const statusPresentation =
    statusKind === 'platform'
      ? {
          tone: tenantStatusTone(Number(account.status) as TenantStatus),
          label: tenantStatusLabel(Number(account.status) as TenantStatus),
        }
      : {
          tone: accountStatusTone(account.status),
          label: accountStatusLabel(account.status),
        }

  /**
   * 身份区的关键属性横排。
   * 平台账号不返回编号/职称/手机号(getPlatformMe 只填 id/name/roles/status),缺失即不渲染空项 ——
   * 属性表里出现一排「—」会让人以为资料没填全。
   */
  const identityProperties = useMemo(() => {
    const items: { label: string; value: ReactNode }[] = []
    if (account.no) {
      items.push({
        label: account.base_identity ? `${baseIdentityLabel(account.base_identity)}编号` : '编号',
        value: <span className="font-mono">{account.no}</span>,
      })
    }
    if (account.title) items.push({ label: '职称', value: account.title })
    if (account.phone_masked) {
      items.push({ label: '手机号', value: <span className="font-mono">{account.phone_masked}</span> })
    }
    if (account.created_at) {
      items.push({ label: '账号创建', value: formatDateTime(account.created_at) })
    }
    return items
  }, [account])

  return (
    <PageScaffold>
      {/*
        归族:详情族(§6.5.3 第 ④)。这一页核对的对象就是「我的账号」——
        h1 由 ObjectIdentity 的姓名承担,故不再出 PageHeader:
        个人中心从顶栏头像菜单进入,没有父级可挂面包屑,空页面头只会白占一行(§6.5.0 通则 1)。
        改密与换绑手机号是表单类动作区(§6.5.4),留在右栏各成一块可聚焦抬起片。
      */}
      <ObjectIdentity
        name={account.name}
        status={<StatusIndicator tone={statusPresentation.tone} label={statusPresentation.label} />}
        subtitle={`${userRolesLabel(account.roles) || '未分配角色'} · 资料由学校维护,如需更正请联系管理员`}
        properties={identityProperties}
      />

      <PageBody
        className="mt-6"
        rail={
          <div className="flex flex-col gap-4">
            <PasswordCard />
            {canChangePhone ? <PhoneCard onChanged={reload} /> : null}
          </div>
        }
      >
        <SessionsSection />
      </PageBody>
    </PageScaffold>
  )
}

/**
 * PasswordCard 修改密码。
 * 改密成功后服务端会吊销其他会话,故明确告知用户其他设备需要重新登录。
 */
function PasswordCard() {
  const fieldId = useId()
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const { errors, setError } = useFieldErrors()
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [done, setDone] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const oldOk = setError('old', requiredError(oldPassword, '请输入当前密码'))
      const newOk = setError('new', passwordRuleError(newPassword, '新密码'))
      const confirmOk = setError(
        'confirm',
        confirmPasswordError(confirmPassword, newPassword, '新密码'),
      )
      if (!oldOk || !newOk || !confirmOk) return

      setSubmitting(true)
      setFormError(null)
      try {
        await api.identity.changePassword({ old_password: oldPassword, new_password: newPassword })
        setOldPassword('')
        setNewPassword('')
        setConfirmPassword('')
        setDone(true)
        toast.success('密码已更新')
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '密码修改失败,请确认当前密码后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [confirmPassword, newPassword, oldPassword, setError],
  )

  return (
    <Card>
      <CardHeader
        title={
          <span className="flex items-center gap-2">
            <KeyRound aria-hidden="true" className="size-4 shrink-0 text-primary" />
            修改密码
          </span>
        }
        description="修改后其他设备上的登录会失效,需要重新登录。"
      />
      <CardBody>
        {done ? (
          <Callout tone="success" title="密码已更新">
            请在其他设备上使用新密码重新登录。
          </Callout>
        ) : null}
        <form onSubmit={submit} noValidate className={done ? 'mt-3' : undefined}>
          <FormField label="当前密码" htmlFor={`${fieldId}-old`} required error={errors.old}>
            <Input
              id={`${fieldId}-old`}
              type="password"
              autoComplete="current-password"
              value={oldPassword}
              invalid={Boolean(errors.old)}
              onChange={(event) => setOldPassword(event.target.value)}
              onBlur={() => setError('old', requiredError(oldPassword, '请输入当前密码'))}
            />
          </FormField>
          <FormField
            label="新密码"
            htmlFor={`${fieldId}-new`}
            required
            error={errors.new}
            className="mt-4"
          >
            <Input
              id={`${fieldId}-new`}
              type="password"
              autoComplete="new-password"
              value={newPassword}
              invalid={Boolean(errors.new)}
              onChange={(event) => setNewPassword(event.target.value)}
              onBlur={() => setError('new', passwordRuleError(newPassword, '新密码'))}
            />
          </FormField>
          <FormField
            label="确认新密码"
            htmlFor={`${fieldId}-confirm`}
            required
            error={errors.confirm}
            className="mt-4"
          >
            <Input
              id={`${fieldId}-confirm`}
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              invalid={Boolean(errors.confirm)}
              onChange={(event) => setConfirmPassword(event.target.value)}
              onBlur={() =>
                setError('confirm', confirmPasswordError(confirmPassword, newPassword, '新密码'))
              }
            />
          </FormField>
          {formError ? (
            <Callout tone="danger" className="mt-3">
              {formError}
            </Callout>
          ) : null}
          <Button type="submit" variant="primary" loading={submitting} className="mt-4 w-full">
            保存新密码
          </Button>
        </form>
      </CardBody>
    </Card>
  )
}

/**
 * PhoneCard 换绑手机号(仅租户账号)。
 * 换绑后资料里的掩码手机号随之更新,故成功后刷新会话并广播资料失效。
 */
function PhoneCard({ onChanged }: { onChanged: () => void }) {
  const fieldId = useId()
  const [phone, setPhone] = useState('')
  const [smsCode, setSmsCode] = useState('')
  const { errors, setError } = useFieldErrors()
  const [formError, setFormError] = useState<string | null>(null)
  const [sendingSms, setSendingSms] = useState(false)
  const { seconds: cooldown, start: startCooldown } = useSmsCooldown()
  const [submitting, setSubmitting] = useState(false)

  /** sendSms 发送换绑场景验证码。 */
  const sendSms = useCallback(async () => {
    if (!setError('phone', phoneError(phone, '新手机号'))) return
    setSendingSms(true)
    setFormError(null)
    try {
      await api.identity.sendSMS({ phone: phone.trim(), scene: SmsScene.CHANGE_PHONE })
      startCooldown()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '验证码发送失败,请稍后重试。'))
    } finally {
      setSendingSms(false)
    }
  }, [phone, setError, startCooldown])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const phoneOk = setError('phone', phoneError(phone, '新手机号'))
      const codeOk = setError('code', requiredError(smsCode, '请输入验证码'))
      if (!phoneOk || !codeOk) return

      setSubmitting(true)
      setFormError(null)
      try {
        await api.identity.changePhone({ phone: phone.trim(), code: smsCode.trim() })
        setPhone('')
        setSmsCode('')
        toast.success('手机号已更新')
        onChanged()
        invalidateAppResource('profile')
      } catch (error) {
        setFormError(userFacingErrorMessage(error, '手机号换绑失败,请确认验证码后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [onChanged, phone, setError, smsCode],
  )

  return (
    <Card>
      <CardHeader
        title={
          <span className="flex items-center gap-2">
            <Smartphone aria-hidden="true" className="size-4 shrink-0 text-primary" />
            换绑手机号
          </span>
        }
        description="手机号用于登录和找回密码,换绑需要新手机号的验证码。"
      />
      <CardBody>
        <form onSubmit={submit} noValidate>
          <FormField label="新手机号" htmlFor={`${fieldId}-phone`} required error={errors.phone}>
            <Input
              id={`${fieldId}-phone`}
              type="tel"
              inputMode="numeric"
              autoComplete="tel"
              value={phone}
              invalid={Boolean(errors.phone)}
              onChange={(event) => setPhone(event.target.value)}
              onBlur={() => setError('phone', phoneError(phone, '新手机号'))}
            />
          </FormField>
          <FormField label="验证码" htmlFor={`${fieldId}-code`} required error={errors.code} className="mt-4">
            <div className="flex items-center gap-2">
              <Input
                id={`${fieldId}-code`}
                inputMode="numeric"
                autoComplete="one-time-code"
                value={smsCode}
                invalid={Boolean(errors.code)}
                onChange={(event) => setSmsCode(event.target.value)}
                onBlur={() => setError('code', requiredError(smsCode, '请输入验证码'))}
              />
              <Button
                type="button"
                variant="outline"
                loading={sendingSms}
                disabled={cooldown > 0}
                onClick={() => void sendSms()}
              >
                {cooldown > 0 ? `${cooldown} 秒后重发` : '获取验证码'}
              </Button>
            </div>
          </FormField>
          {formError ? (
            <Callout tone="danger" className="mt-3">
              {formError}
            </Callout>
          ) : null}
          <Button type="submit" variant="primary" loading={submitting} className="mt-4 w-full">
            确认换绑
          </Button>
        </form>
      </CardBody>
    </Card>
  )
}

/**
 * SessionsSection 列出登录会话。
 * 会话是安全信息:生效中的会话要能被用户认出来(设备与地址),异常会话经改密统一吊销 ——
 * 后端未提供单条会话吊销接口,故不做「踢下线」按钮,而是指引改密。
 */
function SessionsSection() {
  const sessions = useAsyncResource(() => api.identity.getSessions(), [])

  const columns: TableColumn<Session>[] = [
    {
      key: 'device_info',
      header: '设备',
      render: (session) => (
        <span className="text-ink">{session.device_info || '未知设备'}</span>
      ),
    },
    {
      key: 'ip',
      header: '登录地址',
      mono: true,
      render: (session) => session.ip || '—',
    },
    {
      key: 'created_at',
      header: '登录时间',
      render: (session) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(session.created_at)}
        </span>
      ),
    },
    {
      key: 'expire_at',
      header: '有效期至',
      render: (session) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDateTime(session.expire_at)}
        </span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (session) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <StatusIndicator
            tone={sessionStatusTone(session.status)}
            label={sessionStatusLabel(session.status)}
          />
          {session.status === SessionStatus.ACTIVE ? <Badge tone="jade">在线</Badge> : null}
        </div>
      ),
    },
  ]

  return (
    // 列表型页内子视图走 DataPanel 片段(§6.5.5 B):会话清单一次回齐,不分页也不筛选
    <PageSection
      title="登录会话"
      description="这里列出你的登录记录。如果看到不认识的设备,请立即修改密码。"
    >
      <div className="flex flex-col gap-4">
        <Callout tone="info" title="如何退出其他设备">
          <span className="flex items-start gap-2">
            <LogOut aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
            <span>修改密码后,其他设备上的登录会全部失效。</span>
          </span>
        </Callout>

        <DataPanel label="登录会话">
          <ResourceState
            resource={sessions}
            emptyIcon={ShieldCheck}
            emptyTitle="暂无登录记录"
            emptyDescription="登录记录会在你从新设备登录后出现。"
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading elevated={false} />}
          >
            {(list) => (
              <Table
                columns={columns}
                data={list}
                rowKey={(session) => session.id}
                elevated={false}
                // <md 换行卡(§6.4.1 规则 3):设备一行、地址与时间一行
                mobileCard={(session) => ({
                  title: session.device_info || '未知设备',
                  meta: `${session.ip || '地址未知'} · ${formatDateTime(session.created_at)}`,
                  badge: (
                    <StatusIndicator
                      tone={sessionStatusTone(session.status)}
                      label={sessionStatusLabel(session.status)}
                    />
                  ),
                })}
              />
            )}
          </ResourceState>
        </DataPanel>
      </div>
    </PageSection>
  )
}
