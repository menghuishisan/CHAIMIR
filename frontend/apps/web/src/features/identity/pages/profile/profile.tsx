// 个人中心页(共享入口,{prefix}/profile,顶栏头像菜单进入)。
//
// 能力按身份分化(后端 api_me.go:四条路由都只挂 authn.Middleware(),分叉在 service 层):
//   租户账号(学生/教师/校管)= 资料 + 改密 + 换绑手机号 + 登录会话
//   平台账号               = 资料只读 + 改密 + 登录会话(ChangeMyPhone 拒平台身份,
//                            platform_admin 表也没有手机号/学工号/职称/租户列)
// 是否渲染换绑手机号由 props 显式声明,页面不在运行时判角色枚举 —— 与铃铛归属同一处理。

import { useCallback, useId, useMemo, useState, type ReactNode } from 'react'
import { ChevronDown, KeyRound, LogOut, Monitor, ShieldCheck, Smartphone } from 'lucide-react'
import { SessionStatus, SmsScene, TenantStatus, type Session } from '@chaimir/api-client'
import {
  Badge,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  cn,
  FormField,
  Input,
  PageHeader,
  PageScaffold,
  Skeleton,
  StatusIndicator,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { invalidateAppResource } from '../../../../app/resourceInvalidation'
import { ResourceError } from '../../../../components/ResourceState'
import { useSession } from '../../../../components/RoleGuard'
import { useAsyncResource, usePageLocation } from '../../../../hooks'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { deviceLabel } from '../../deviceLabel'
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
  useSmsCooldown,
} from '../../authForm'
import { useFormValidation } from '../../../../hooks/useFormValidation'

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

  usePageLocation([{ label: '个人中心' }])

  return (
    <PageScaffold>
      <PageHeader
        title={account.name}
        status={<StatusIndicator tone={statusPresentation.tone} label={statusPresentation.label} />}
        meta={[{ value: userRolesLabel(account.roles) || '未分配角色' }, ...identityProperties]}
      />

      <div className="grid gap-6 lg:grid-cols-5">
        <div className="min-w-0 lg:col-span-3">
          <SessionsSection />
        </div>
        <div className="flex min-w-0 flex-col gap-5 lg:col-span-2">
          <PasswordCard />
          {canChangePhone ? <PhoneCard onChanged={reload} /> : null}
          <p className="text-sm text-ink-sub">
            {canChangePhone
              ? '姓名、编号等身份资料由学校维护,如需更正请联系学校管理员。'
              : '平台账号的资料由系统管理员维护,如需更正请联系他们。'}
          </p>
        </div>
      </div>
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
  const { errors, validate, reset } = useFormValidation(() => ({
    old: requiredError(oldPassword, '请输入当前密码'),
    new: passwordRuleError(newPassword, '新密码'),
    confirm: confirmPasswordError(confirmPassword, newPassword, '新密码'),
  }))
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [done, setDone] = useState(false)

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return

      setSubmitting(true)
      setFormError(null)
      try {
        await api.identity.changePassword({ old_password: oldPassword, new_password: newPassword })
        reset()
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
    [newPassword, oldPassword, reset, validate],
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
              onBlur={() => validate(['old'])}
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
              onBlur={() => validate(['new'])}
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
              onBlur={() => validate(['confirm'])}
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
  const { errors, validate, reset } = useFormValidation(() => ({
    phone: phoneError(phone, '新手机号'),
    code: requiredError(smsCode, '请输入验证码'),
  }))
  const [formError, setFormError] = useState<string | null>(null)
  const [sendingSms, setSendingSms] = useState(false)
  const { seconds: cooldown, start: startCooldown } = useSmsCooldown()
  const [submitting, setSubmitting] = useState(false)

  /** sendSms 发送换绑场景验证码。 */
  const sendSms = useCallback(async () => {
    if (!validate(['phone'])) return
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
  }, [phone, startCooldown, validate])

  const submit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate()) return

      setSubmitting(true)
      setFormError(null)
      try {
        await api.identity.changePhone({ phone: phone.trim(), code: smsCode.trim() })
        reset()
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
    [onChanged, phone, reset, smsCode, validate],
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
              onBlur={() => validate(['phone'])}
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
                onBlur={() => validate(['code'])}
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
 * SessionsSection 登录设备:每个会话一行,显示为「Windows · Chrome 152」(前端解析浏览器信息串,
 * 完整信息串悬停可见)、登录地址、登录时间、状态;正在用的这台设备排第一并标「这台设备」
 * (由服务端按本次请求的会话判定,前端不凭设备信息猜),其余有效会话标「在线」,已退出的折叠。
 * 后端没有单条会话吊销接口,退出其他设备靠修改密码。
 */
function SessionsSection() {
  const sessions = useAsyncResource(() => api.identity.getSessions(), [])
  const [showEnded, setShowEnded] = useState(false)
  const list = sessions.data ?? []
  const active = list
    .filter((session) => session.status === SessionStatus.ACTIVE)
    .sort((a, b) => Number(b.current) - Number(a.current) || b.created_at.localeCompare(a.created_at))
  const ended = list
    .filter((session) => session.status !== SessionStatus.ACTIVE)
    .sort((a, b) => b.created_at.localeCompare(a.created_at))

  return (
    <section aria-labelledby="sessions-title">
      <h2 id="sessions-title" className="text-md font-semibold text-ink">
        登录设备
      </h2>
      <p className="mb-3 mt-1 flex items-start gap-1.5 text-sm text-ink-sub">
        <LogOut aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
        看到不认识的设备,请立即修改密码 —— 改密后其他设备上的登录会全部失效。
      </p>
      {sessions.status === 'error' ? (
        <ResourceError
          message={sessions.error?.message ?? '登录记录暂时读取不到'}
          traceId={sessions.error?.traceId}
          onRetry={sessions.reload}
        />
      ) : sessions.status === 'loading' ? (
        <div aria-busy="true" className="rounded-lg bg-surface p-4 shadow-xs">
          <Skeleton variant="line" lines={4} />
        </div>
      ) : list.length === 0 ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-12 text-center shadow-xs">
          <ShieldCheck aria-hidden="true" className="size-6 text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">还没有登录记录</p>
        </div>
      ) : (
        <>
          <ul className="divide-y divide-line overflow-hidden rounded-lg bg-surface shadow-xs">
            {active.map((session) => (
              <li key={session.id}>
                <SessionRow session={session} />
              </li>
            ))}
            {active.length === 0 ? <li className="px-4 py-3 text-sm text-ink-sub">没有正在登录的设备。</li> : null}
          </ul>
          {ended.length > 0 ? (
            <div className="mt-4">
              <button
                type="button"
                aria-expanded={showEnded}
                onClick={() => setShowEnded((value) => !value)}
                className="flex items-center gap-1.5 rounded-md text-sm font-medium text-ink-sub hover:text-ink focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
              >
                <ChevronDown aria-hidden="true" className={cn('size-4 transition-transform duration-fast', showEnded ? '' : '-rotate-90')} />
                已退出的设备({ended.length})
              </button>
              {showEnded ? (
                <ul className="mt-3 divide-y divide-line overflow-hidden rounded-lg bg-surface shadow-xs">
                  {ended.map((session) => (
                    <li key={session.id}>
                      <SessionRow session={session} />
                    </li>
                  ))}
                </ul>
              ) : null}
            </div>
          ) : null}
        </>
      )}
    </section>
  )
}

/** SessionRow 一个登录会话:设备名(完整信息串悬停可见)、地址与时间、状态;正在用的这台设备单独标出 */
function SessionRow({ session }: { session: Session }) {
  const online = session.status === SessionStatus.ACTIVE
  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <span className="grid size-9 shrink-0 place-items-center rounded-md bg-surface-hover text-ink-sub">
        <Monitor aria-hidden="true" className="size-4" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-base font-medium text-ink" title={session.device_info || undefined}>
          {deviceLabel(session.device_info)}
        </p>
        <p className="truncate text-sm text-ink-sub">
          <span className="font-mono">{session.ip || '地址未知'}</span>
          {' · '}
          <time dateTime={session.created_at} title={formatDateTime(session.created_at)}>
            {formatRelativeOccurrence(session.created_at)}
          </time>
          登录
        </p>
      </div>
      {online && session.current ? (
        <Badge tone="jade">这台设备</Badge>
      ) : online ? (
        <Badge tone="neutral">在线</Badge>
      ) : (
        <StatusIndicator tone={sessionStatusTone(session.status)} label={sessionStatusLabel(session.status)} />
      )}
    </div>
  )
}
