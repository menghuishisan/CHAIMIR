// LoginPage 以手机号登录为主流程，并提供校内账号和学校统一认证次级入口。

import React, { useCallback, useEffect, useState } from 'react'
import type { LoginResponse, TenantOption } from '@chaimir/api-client'
import { SmsScene } from '@chaimir/api-client'
import { Button, Checkbox, FormField, Input, SegmentedControl } from '@chaimir/ui'
import { ArrowLeft, Building2, IdCard } from 'lucide-react'
import { useLocation, useNavigate } from 'react-router-dom'
import { api } from '../../../../app/api'
import { platformLayerEnabled } from '../../../../app/config'
import { loginEntryPath, persistLoginTokens } from '../../../../utils/authSession'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import styles from './auth-form.module.css'

type LoginView = 'phone' | 'account'
type PhoneLoginMethod = 'password' | 'sms'

export interface TenantSelectLoginState {
  tenants: TenantOption[]
  remember: boolean
  returnPath?: string
  method:
    | { type: 'phone'; phone: string; password: string }
    | { type: 'sms'; phone: string; code: string }
}

const PHONE_LOGIN_METHODS = [
  { value: 'password', label: '密码登录' },
  { value: 'sms', label: '验证码登录' },
]

/**
 * LoginPage 处理手机号主登录、多学校选择和低频认证入口跳转。
 */
const LoginPage: React.FC = () => {
  const [view, setView] = useState<LoginView>('phone')
  const [phoneMethod, setPhoneMethod] = useState<PhoneLoginMethod>('password')
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [tenantCode, setTenantCode] = useState('')
  const [accountNo, setAccountNo] = useState('')
  const [smsCode, setSmsCode] = useState('')
  const [remember, setRemember] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [sendingSms, setSendingSms] = useState(false)
  const [smsCountdown, setSmsCountdown] = useState(0)
  const [smsMessage, setSmsMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const location = useLocation()
  const returnPath = (location.state as { from?: string } | null)?.from

  useEffect(() => {
    if (smsCountdown <= 0) return
    const timer = window.setTimeout(() => setSmsCountdown((current) => Math.max(0, current - 1)), 1000)
    return () => window.clearTimeout(timer)
  }, [smsCountdown])

  /**
   * completeLogin 持久化会话，并按服务端改密要求或账号角色决定落点。
   */
  const completeLogin = useCallback((response: LoginResponse) => {
    persistLoginTokens(response, remember)
    navigate(loginEntryPath(response, returnPath), { replace: true })
  }, [navigate, remember, returnPath])

  /**
   * openLoginView 切换手机号与校内账号表单，并清除上一种方式的错误提示。
   */
  const openLoginView = useCallback((nextView: LoginView) => {
    setView(nextView)
    setError(null)
  }, [])

  /**
   * handlePhonePasswordLogin 提交手机号和密码，必要时进入多学校选择。
   */
  const handlePhonePasswordLogin = useCallback(async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!phone.trim() || !password) {
      setError('请输入手机号和密码。')
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      const response = await api.identity.loginPhone({ phone: phone.trim(), password })
      if (response.need_select_tenant && response.tenants?.length) {
        navigate('/auth/tenant-select', {
          state: {
            tenants: response.tenants,
            remember,
            returnPath,
            method: { type: 'phone', phone: phone.trim(), password },
          } satisfies TenantSelectLoginState,
        })
        return
      }
      completeLogin(response)
    } catch (loginError) {
      setError(userFacingErrorMessage(loginError, '登录失败，请检查手机号和密码后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [completeLogin, navigate, password, phone, remember, returnPath])

  /**
   * handleAccountLogin 使用学校代号和校内账号完成登录。
   */
  const handleAccountLogin = useCallback(async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!tenantCode.trim() || !accountNo.trim() || !password) {
      setError('请输入学校代号、学号或工号和密码。')
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      const response = await api.identity.loginNo({
        tenant_code: tenantCode.trim(),
        no: accountNo.trim(),
        password,
      })
      completeLogin(response)
    } catch (loginError) {
      setError(userFacingErrorMessage(loginError, '登录失败，请检查学校代号和账号信息。'))
    } finally {
      setSubmitting(false)
    }
  }, [accountNo, completeLogin, password, tenantCode])

  /**
   * handleSmsLogin 提交手机号和短信验证码，必要时进入多学校选择。
   */
  const handleSmsLogin = useCallback(async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!phone.trim() || !smsCode.trim()) {
      setError('请输入手机号和短信验证码。')
      return
    }
    setSubmitting(true)
    setError(null)
    try {
      const response = await api.identity.loginSMS({ phone: phone.trim(), code: smsCode.trim() })
      if (response.need_select_tenant && response.tenants?.length) {
        navigate('/auth/tenant-select', {
          state: {
            tenants: response.tenants,
            remember,
            returnPath,
            method: { type: 'sms', phone: phone.trim(), code: smsCode.trim() },
          } satisfies TenantSelectLoginState,
        })
        return
      }
      completeLogin(response)
    } catch (loginError) {
      setError(userFacingErrorMessage(loginError, '登录失败，请检查验证码后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [completeLogin, navigate, phone, remember, returnPath, smsCode])

  /**
   * handleSendSms 请求登录验证码，并将服务端失败原因转换为用户向提示。
   */
  const handleSendSms = useCallback(async () => {
    if (smsCountdown > 0) return
    if (!phone.trim()) {
      setError('请先输入手机号。')
      return
    }
    setSendingSms(true)
    setError(null)
    setSmsMessage(null)
    try {
      await api.identity.sendSMS({ phone: phone.trim(), scene: SmsScene.LOGIN })
      setSmsCountdown(60)
      setSmsMessage('验证码已发送，请查看手机短信。')
    } catch (smsError) {
      setError(userFacingErrorMessage(smsError, '验证码发送失败，请稍后重试。'))
    } finally {
      setSendingSms(false)
    }
  }, [phone, smsCountdown])

  return (
    <div className={styles.form}>
      <div>
        <h1 className={styles.title}>{view === 'phone' ? '欢迎回来' : '使用校内账号登录'}</h1>
        <p className={styles.description}>
          {view === 'phone' ? '使用绑定手机号登录，进入您的学习或管理页面。' : '请输入学校代号和学校分配的学号或工号。'}
        </p>
      </div>

      {error && <div className={styles.error} role="alert">{error}</div>}
      {smsMessage && <div className={styles.success} role="status">{smsMessage}</div>}

      {view === 'phone' ? (
        <>
          <SegmentedControl
            className={styles.phoneMethods}
            options={PHONE_LOGIN_METHODS}
            value={phoneMethod}
            label="手机号登录方式"
            onChange={(value) => {
              setPhoneMethod(value as PhoneLoginMethod)
              setError(null)
            }}
          />

          {phoneMethod === 'password' ? (
            <form className={styles.fields} onSubmit={handlePhonePasswordLogin}>
              <FormField label="手机号" htmlFor="login-phone" required>
                <Input id="login-phone" fullWidth inputMode="tel" autoComplete="tel" placeholder="例如 13800000000" value={phone} onChange={(event) => setPhone(event.target.value)} />
              </FormField>
              <FormField label="密码" htmlFor="login-phone-password" required>
                <Input id="login-phone-password" fullWidth type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} />
              </FormField>
              <Button block type="submit" loading={submitting}>登录</Button>
            </form>
          ) : (
            <form className={styles.fields} onSubmit={handleSmsLogin}>
              <FormField label="手机号" htmlFor="login-sms-phone" required>
                <Input id="login-sms-phone" fullWidth inputMode="tel" autoComplete="tel" placeholder="例如 13800000000" value={phone} onChange={(event) => setPhone(event.target.value)} />
              </FormField>
              <div className={styles.inline}>
                <FormField label="短信验证码" htmlFor="login-sms-code" required>
                  <Input id="login-sms-code" fullWidth autoCapitalize="characters" autoComplete="one-time-code" value={smsCode} onChange={(event) => setSmsCode(event.target.value)} />
                </FormField>
                <Button className={styles.inlineButton} variant="outline" loading={sendingSms} disabled={smsCountdown > 0} onClick={handleSendSms}>{smsCountdown > 0 ? `${smsCountdown} 秒后可重发` : '获取验证码'}</Button>
              </div>
              <Button block type="submit" loading={submitting}>登录</Button>
            </form>
          )}
        </>
      ) : (
        <form className={styles.fields} onSubmit={handleAccountLogin}>
          <FormField label="学校代号" htmlFor="login-tenant-code" helperText="由学校管理员提供" required>
            <Input id="login-tenant-code" fullWidth autoComplete="organization" value={tenantCode} onChange={(event) => setTenantCode(event.target.value)} />
          </FormField>
          <FormField label="学号或工号" htmlFor="login-account-no" required>
            <Input id="login-account-no" fullWidth autoComplete="username" value={accountNo} onChange={(event) => setAccountNo(event.target.value)} />
          </FormField>
          <FormField label="密码" htmlFor="login-account-password" required>
            <Input id="login-account-password" fullWidth type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} />
          </FormField>
          <Button block type="submit" loading={submitting}>登录</Button>
          <Button block variant="ghost" icon={<ArrowLeft size={16} />} onClick={() => openLoginView('phone')}>返回手机号登录</Button>
        </form>
      )}

      <div className={styles.options}>
        <Checkbox checked={remember} label="保持登录" onChange={(event) => setRemember(event.target.checked)} />
        <button className={styles.linkButton} type="button" onClick={() => navigate('/auth/forgot')}>忘记密码</button>
      </div>

      {view === 'phone' ? (
        <div className={styles.otherLogin}>
          <span className={styles.otherLoginLabel}>其他登录方式</span>
          <div className={styles.otherLoginActions}>
            <Button block variant="outline" icon={<IdCard size={16} />} onClick={() => openLoginView('account')}>学号或工号</Button>
            <Button block variant="outline" icon={<Building2 size={16} />} onClick={() => navigate(returnPath ? `/auth/sso?return_to=${encodeURIComponent(returnPath)}` : '/auth/sso')}>学校统一认证</Button>
          </div>
        </div>
      ) : null}

      <div className={styles.footerLinks}>
        <span>尚未入驻？ <button className={styles.linkButton} type="button" onClick={() => navigate('/auth/apply')}>申请入驻平台</button></span>
        <span>首次使用？ <button className={styles.linkButton} type="button" onClick={() => navigate('/auth/activate')}>账号激活</button></span>
      </div>

      {platformLayerEnabled ? (
        <div className={styles.platformEntry}>
          <button className={styles.linkButton} type="button" onClick={() => navigate('/auth/platform-login')}>平台管理员登录</button>
        </div>
      ) : null}
    </div>
  )
}

export default LoginPage
