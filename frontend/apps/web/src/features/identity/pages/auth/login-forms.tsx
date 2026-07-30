// login-forms 是登录页三种登录方式的表单:手机号(密码/验证码)、校内账号、学校统一认证。
// 三者共用同一个状态机(login-state 的 LoginController),因此切换方式不会丢已填内容;
// 每种方式一个表单组件,提交动作与字段一一对应,不做「一个表单按变量长出不同字段」的分支拼装。
// 页尾三条次级公共入口(统一认证 / 激活码 / 入驻申请)随主要方式一同出现,统一认证页自身不再重复。

import { useId } from 'react'
import { Link } from 'react-router-dom'
import { ArrowLeft, IdCard } from 'lucide-react'
import { Checkbox, cn } from '@chaimir/ui'
import { platformLayerEnabled } from '../../../../app/config'
import {
  AuthFormError,
  AuthHeading,
  AuthQuietAction,
  AuthSmsField,
  AuthSubmit,
  AuthTextField,
} from './auth-ui'
import type { LoginController, PhoneLoginMethod } from './login-state'

/** 手机号登录的两种凭据方式(页面级双态开关的选项) */
const PHONE_METHODS: { value: PhoneLoginMethod; label: string }[] = [
  { value: 'password', label: '密码登录' },
  { value: 'sms', label: '验证码登录' },
]

/**
 * PhoneMethodSwitch 密码/验证码方式切换。
 * 手写而非用设计系统 SegmentedControl:后者是光面语境配色,放到墨色底层上对比关系会反过来。
 */
function PhoneMethodSwitch({
  method,
  onChange,
}: {
  method: PhoneLoginMethod
  onChange: (next: PhoneLoginMethod) => void
}) {
  return (
    <div className="mt-6 flex gap-1 rounded-lg bg-dark-elevated p-1" role="group" aria-label="登录方式">
      {PHONE_METHODS.map((option) => (
        <button
          key={option.value}
          type="button"
          aria-pressed={method === option.value}
          onClick={() => onChange(option.value)}
          className={cn(
            'flex-1 rounded-md py-1.5 text-sm transition-colors duration-fast',
            method === option.value
              ? 'bg-dark-surface font-medium text-accent'
              : 'text-on-dark-sub hover:text-on-dark',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  )
}

/**
 * RememberRow 免登录选项与忘记密码入口同行:一个决定这次登录记多久,一个是登不进去时的出路。
 */
function RememberRow({ remember, onRememberChange }: { remember: boolean; onRememberChange: (value: boolean) => void }) {
  return (
    <div className="mt-5 flex items-center justify-between">
      <Checkbox
        label="30 天内免登录"
        checked={remember}
        onCheckedChange={(checked) => onRememberChange(checked === true)}
        className="text-on-dark-sub"
      />
      <Link to="/auth/forgot" className="text-sm text-on-dark-sub transition-colors duration-fast hover:text-accent">
        忘记密码
      </Link>
    </div>
  )
}

/**
 * AltEntries 登录页底部的次级公共入口。
 * 这三条通路都对应后端免认证接口,若不在登录页给出落点,用户就只能靠手输地址进入:
 * 统一认证(CAS/LDAP)、激活码开通账号、学校入驻申请(仅 SaaS 形态存在)。
 */
function AltEntries({ onUseSso }: { onUseSso: () => void }) {
  return (
    <div className="mt-10">
      {/* 与主表单的分界靠留白密度,不画分隔线(FE-6/规范 §1.2) */}
      <p className="text-xs uppercase tracking-widest text-on-dark-faint">其他方式</p>
      <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-on-dark-sub">
        <button type="button" onClick={onUseSso} className="transition-colors duration-fast hover:text-accent">
          学校统一认证登录
        </button>
        <span aria-hidden className="text-on-dark-faint">
          ·
        </span>
        <Link to="/auth/activate" className="transition-colors duration-fast hover:text-accent">
          用激活码开通账号
        </Link>
        {platformLayerEnabled ? (
          <>
            <span aria-hidden className="text-on-dark-faint">
              ·
            </span>
            <Link to="/auth/apply" className="transition-colors duration-fast hover:text-accent">
              学校入驻申请
            </Link>
          </>
        ) : null}
      </div>
    </div>
  )
}

/**
 * PhoneLoginForm 手机号登录(主要方式):密码或短信验证码二选一。
 */
export function PhoneLoginForm({ login }: { login: LoginController }) {
  const fieldId = useId()
  return (
    <form onSubmit={login.submitPhone} noValidate>
      <AuthHeading title="欢迎回来" description="使用绑定的手机号登录" />

      <PhoneMethodSwitch method={login.phoneMethod} onChange={login.setPhoneMethod} />

      <AuthTextField
        label="手机号"
        id={`${fieldId}-phone`}
        type="tel"
        autoComplete="tel"
        inputMode="numeric"
        value={login.phone}
        error={login.errors.phone}
        onValueChange={login.setPhone}
        onBlur={login.blurPhone}
      />

      {login.phoneMethod === 'password' ? (
        <AuthTextField
          label="密码"
          id={`${fieldId}-password`}
          type="password"
          autoComplete="current-password"
          value={login.password}
          error={login.errors.password}
          onValueChange={login.setPassword}
          onBlur={login.blurPassword}
        />
      ) : (
        <AuthSmsField
          id={`${fieldId}-sms`}
          value={login.smsCode}
          error={login.errors.smsCode}
          onValueChange={login.setSmsCode}
          onBlur={login.blurSmsCode}
          cooldownSeconds={login.smsCooldown}
          sending={login.sendingSms}
          onSend={login.sendSms}
        />
      )}

      <RememberRow remember={login.remember} onRememberChange={login.setRemember} />

      <AuthFormError message={login.formError} />

      <AuthSubmit loading={login.submitting}>登录</AuthSubmit>

      <AuthQuietAction icon={IdCard} onClick={() => login.switchView('account')}>
        使用学校代号和学号/工号登录
      </AuthQuietAction>

      <AltEntries onUseSso={() => login.switchView('sso')} />
    </form>
  )
}

/**
 * AccountLoginForm 校内账号登录(次级方式):学校代号 + 学号/工号 + 密码。
 */
export function AccountLoginForm({ login }: { login: LoginController }) {
  const fieldId = useId()
  return (
    <form onSubmit={login.submitAccount} noValidate>
      <AuthHeading title="校内账号登录" description="使用学校下发的账号登录" />

      <AuthTextField
        label="学校代号"
        id={`${fieldId}-tenant`}
        autoComplete="organization"
        value={login.tenantCode}
        error={login.errors.tenantCode}
        onValueChange={login.setTenantCode}
        onBlur={login.blurTenantCode}
      />

      <AuthTextField
        label="学号 / 工号"
        id={`${fieldId}-no`}
        autoComplete="username"
        value={login.accountNo}
        error={login.errors.accountNo}
        onValueChange={login.setAccountNo}
        onBlur={login.blurAccountNo}
      />

      <AuthTextField
        label="密码"
        id={`${fieldId}-password`}
        type="password"
        autoComplete="current-password"
        value={login.password}
        error={login.errors.password}
        onValueChange={login.setPassword}
        onBlur={login.blurPassword}
      />

      <RememberRow remember={login.remember} onRememberChange={login.setRemember} />

      <AuthFormError message={login.formError} />

      <AuthSubmit loading={login.submitting}>登录</AuthSubmit>

      <AuthQuietAction icon={ArrowLeft} onClick={() => login.switchView('phone')}>
        返回手机号登录
      </AuthQuietAction>

      <AltEntries onUseSso={() => login.switchView('sso')} />
    </form>
  )
}

/**
 * SsoLoginForm 学校统一认证入口:只收学校代号,认证本身在该校认证系统完成。
 * 这里不列其他方式入口 —— 下一步已经明确是离站认证,再铺开选项会岔开注意力。
 */
export function SsoLoginForm({ login }: { login: LoginController }) {
  const fieldId = useId()
  return (
    <form onSubmit={login.submitSso} noValidate>
      <AuthHeading title="学校统一认证" description="输入学校代号后前往本校认证系统登录" />

      <AuthTextField
        label="学校代号"
        id={`${fieldId}-tenant`}
        autoComplete="organization"
        value={login.tenantCode}
        error={login.errors.tenantCode}
        onValueChange={login.setTenantCode}
        onBlur={login.blurTenantCode}
      />

      <AuthFormError message={login.formError} />

      <AuthSubmit>继续</AuthSubmit>

      <AuthQuietAction icon={ArrowLeft} onClick={() => login.switchView('phone')}>
        返回手机号登录
      </AuthQuietAction>
    </form>
  )
}
