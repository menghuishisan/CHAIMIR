// login-forms 是登录页三种登录方式的表单:手机号(密码/验证码)、校内账号、学校统一认证。
// 三者共用同一个状态机(login-state 的 LoginController),因此切换方式不会丢已填内容;
// 每种方式一个表单组件,提交动作与字段一一对应,不做「一个表单按变量长出不同字段」的分支拼装。
// 布局(设计 §7.1):主表单为手机号登录,顶部分段切换「密码 / 验证码」;一条分隔线引出「其他登录方式」,
// 并列「校内账号」「学校统一认证」两个次要按钮;最下方一行「首次使用?激活账号 · 忘记密码 · 学校申请入驻」。

import { useId } from 'react'
import { Link } from 'react-router'
import { ArrowLeft, Building2, IdCard } from 'lucide-react'
import { Button, Checkbox } from '@chaimir/ui'
import { platformLayerEnabled } from '../../../../app/config'
import {
  AUTH_TEXT_LINK_CLASS,
  AuthFormError,
  AuthHeading,
  AuthLinkRow,
  AuthQuietAction,
  AuthSegmented,
  AuthSmsField,
  AuthSubmit,
  AuthTextField,
} from '../../components/AuthUI'
import type { LoginController, PhoneLoginMethod } from './login-state'

/** 手机号登录的两种凭据方式 */
const PHONE_METHODS: { value: PhoneLoginMethod; label: string }[] = [
  { value: 'password', label: '密码' },
  { value: 'sms', label: '验证码' },
]

/**
 * RememberRow 免登录选项放在主按钮上方左侧:它决定这次登录记多久。
 */
function RememberRow({ remember, onRememberChange }: { remember: boolean; onRememberChange: (value: boolean) => void }) {
  return (
    <div className="mt-5 flex items-center">
      <Checkbox
        label="30 天内免登录"
        checked={remember}
        onCheckedChange={(checked) => onRememberChange(checked === true)}
        onDark
        className="text-on-dark-sub"
      />
    </div>
  )
}

/**
 * OtherMethods 一条分隔线引出「其他登录方式」,并列校内账号与学校统一认证两个次要按钮。
 * 点击在同一块区域切换表单(登录方式写进地址,刷新不回到默认)。
 */
function OtherMethods({ login }: { login: LoginController }) {
  return (
    <div className="mt-8">
      <div className="flex items-center gap-3 text-sm text-on-dark-sub">
        <span aria-hidden="true" className="h-px flex-1 bg-dark-line" />
        <span>其他登录方式</span>
        <span aria-hidden="true" className="h-px flex-1 bg-dark-line" />
      </div>
      <div className="mt-4 grid grid-cols-2 gap-3">
        <Button variant="on-dark" leftIcon={IdCard} onClick={() => login.switchView('account')}>
          校内账号
        </Button>
        <Button variant="on-dark" leftIcon={Building2} onClick={() => login.switchView('sso')}>
          学校统一认证
        </Button>
      </div>
    </div>
  )
}

/**
 * EntryLinks 最下方一行公共入口,按使用频率排列:激活账号、忘记密码、学校申请入驻(仅 SaaS)。
 * 这些通路都对应后端免认证接口,若不在登录页给出落点,用户就只能靠手输地址进入。
 */
function EntryLinks() {
  return (
    <AuthLinkRow lead="首次使用?">
      <Link to="/auth/activate" className={AUTH_TEXT_LINK_CLASS}>
        激活账号
      </Link>
      <Link to="/auth/forgot" className={AUTH_TEXT_LINK_CLASS}>
        忘记密码
      </Link>
      {platformLayerEnabled ? (
        <Link to="/auth/apply" className={AUTH_TEXT_LINK_CLASS}>
          学校申请入驻
        </Link>
      ) : null}
    </AuthLinkRow>
  )
}

/**
 * PhoneLoginForm 手机号登录(主要方式):密码或短信验证码二选一。
 */
export function PhoneLoginForm({ login }: { login: LoginController }) {
  const fieldId = useId()
  return (
    <form onSubmit={login.submitPhone} noValidate>
      <AuthHeading title="登录 Chaimir" description="用绑定的手机号登录" />

      <AuthSegmented label="登录凭据" options={PHONE_METHODS} value={login.phoneMethod} onChange={login.setPhoneMethod} />

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

      <OtherMethods login={login} />

      <EntryLinks />
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
      <AuthHeading title="校内账号登录" description="用学校下发的学号或工号登录" />

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

      <EntryLinks />
    </form>
  )
}

/**
 * SsoLoginForm 学校统一认证入口:只收学校代号,认证本身在该校认证系统完成。
 * 这里不列其他公共入口 —— 下一步已经明确是去本校认证,再铺开选项会岔开注意力。
 */
export function SsoLoginForm({ login }: { login: LoginController }) {
  const fieldId = useId()
  return (
    <form onSubmit={login.submitSso} noValidate>
      <AuthHeading title="学校统一认证" description="输入学校代号后,前往本校的认证系统登录" />

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
