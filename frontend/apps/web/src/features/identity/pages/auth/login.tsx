// LoginPage 提供本地账号、手机号密码和短信验证码登录，调用 identity 后端模块。

import React, { useCallback, useState } from 'react'
import type { ApiError, LoginResponse, TenantOption } from '@chaimir/api-client'
import { SmsScene } from '@chaimir/api-client'
import { Button, Input } from '@chaimir/ui'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../app/api'
import { persistLoginTokens, roleEntryPath } from '../../../../utils/authSession'
import styles from './auth-form.module.css'

type LoginTab = 'phone' | 'account' | 'sms'

export interface TenantSelectLoginState {
  tenants: TenantOption[]
  remember: boolean
  method:
    | { type: 'phone'; phone: string; password: string }
    | { type: 'sms'; phone: string; code: string }
}

/**
 * LoginPage 处理登录表单提交、短信发送和多租户选择跳转。
 */
const LoginPage: React.FC = () => {
  const [tab, setTab] = useState<LoginTab>('phone')
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [tenantCode, setTenantCode] = useState('')
  const [accountNo, setAccountNo] = useState('')
  const [smsCode, setSmsCode] = useState('')
  const [remember, setRemember] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [sendingSms, setSendingSms] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  const completeLogin = useCallback((response: LoginResponse) => {
    persistLoginTokens(response, remember)
    navigate(roleEntryPath(response), { replace: true })
  }, [navigate, remember])

  const handlePhonePasswordLogin = useCallback(async () => {
    setSubmitting(true)
    setError(null)
    try {
      const response = await api.identity.loginPhone({ phone, password })
      if (response.need_select_tenant && response.tenants?.length) {
        navigate('/auth/tenant-select', {
          state: {
            tenants: response.tenants,
            remember,
            method: { type: 'phone', phone, password },
          } satisfies TenantSelectLoginState,
        })
        return
      }
      completeLogin(response)
    } catch (loginError) {
      setError((loginError as ApiError).message || '登录失败，请检查账号信息后重试')
    } finally {
      setSubmitting(false)
    }
  }, [completeLogin, navigate, password, phone, remember])

  const handleAccountLogin = useCallback(async () => {
    setSubmitting(true)
    setError(null)
    try {
      const response = await api.identity.loginNo({
        tenant_code: tenantCode,
        no: accountNo,
        password,
      })
      completeLogin(response)
    } catch (loginError) {
      setError((loginError as ApiError).message || '登录失败，请检查学校代号和账号信息')
    } finally {
      setSubmitting(false)
    }
  }, [accountNo, completeLogin, password, tenantCode])

  const handleSmsLogin = useCallback(async () => {
    setSubmitting(true)
    setError(null)
    try {
      const response = await api.identity.loginSMS({ phone, code: smsCode })
      if (response.need_select_tenant && response.tenants?.length) {
        navigate('/auth/tenant-select', {
          state: {
            tenants: response.tenants,
            remember,
            method: { type: 'sms', phone, code: smsCode },
          } satisfies TenantSelectLoginState,
        })
        return
      }
      completeLogin(response)
    } catch (loginError) {
      setError((loginError as ApiError).message || '登录失败，请检查验证码后重试')
    } finally {
      setSubmitting(false)
    }
  }, [completeLogin, navigate, phone, remember, smsCode])

  const handleSendSms = useCallback(async () => {
    setSendingSms(true)
    setError(null)
    try {
      await api.identity.sendSMS({ phone, scene: SmsScene.LOGIN })
    } catch (smsError) {
      setError((smsError as ApiError).message || '验证码发送失败，请稍后重试')
    } finally {
      setSendingSms(false)
    }
  }, [phone])

  return (
    <div className={styles.form}>
      <div>
        <h2 className={styles.title}>欢迎回来，请登录</h2>
        <p className={styles.description}>使用平台账号进入对应角色的第一个功能页。</p>
      </div>

      <div className={styles.tabs} role="tablist" aria-label="登录方式">
        {[
          ['phone', '手机号密码'],
          ['account', '学号工号'],
          ['sms', '手机验证码'],
        ].map(([key, label]) => (
          <button
            className={`${styles.tab} ${tab === key ? styles.tabActive : ''}`}
            key={key}
            type="button"
            role="tab"
            aria-selected={tab === key}
            onClick={() => setTab(key as LoginTab)}
          >
            {label}
          </button>
        ))}
      </div>

      {error && <div className={styles.error}>{error}</div>}

      <div className={styles.fields}>
        {tab === 'phone' && (
          <>
            <Input fullWidth placeholder="手机号" value={phone} onChange={(event) => setPhone(event.target.value)} />
            <Input fullWidth type="password" placeholder="密码" value={password} onChange={(event) => setPassword(event.target.value)} />
            <Button block loading={submitting} onClick={handlePhonePasswordLogin}>
              登录
            </Button>
          </>
        )}

        {tab === 'account' && (
          <>
            <Input fullWidth placeholder="学校代号" value={tenantCode} onChange={(event) => setTenantCode(event.target.value)} />
            <Input fullWidth placeholder="学号或工号" value={accountNo} onChange={(event) => setAccountNo(event.target.value)} />
            <Input fullWidth type="password" placeholder="密码" value={password} onChange={(event) => setPassword(event.target.value)} />
            <Button block loading={submitting} onClick={handleAccountLogin}>
              登录
            </Button>
          </>
        )}

        {tab === 'sms' && (
          <>
            <Input fullWidth placeholder="手机号" value={phone} onChange={(event) => setPhone(event.target.value)} />
            <div className={styles.inline}>
              <Input fullWidth placeholder="验证码" value={smsCode} onChange={(event) => setSmsCode(event.target.value)} />
              <Button variant="outline" loading={sendingSms} onClick={handleSendSms}>
                获取验证码
              </Button>
            </div>
            <Button block loading={submitting} onClick={handleSmsLogin}>
              登录
            </Button>
          </>
        )}
      </div>

      <div className={styles.options}>
        <label className={styles.checkbox}>
          <input checked={remember} type="checkbox" onChange={(event) => setRemember(event.target.checked)} />
          保持登录
        </label>
        <button className={styles.linkButton} type="button" onClick={() => navigate('/auth/forgot')}>
          忘记密码
        </button>
      </div>

      <div className={styles.footerLinks}>
        <span>
          尚未入驻？
          <button className={styles.linkButton} type="button" onClick={() => navigate('/auth/apply')}>
            申请入驻平台
          </button>
        </span>
        <span>
          首次使用？
          <button className={styles.linkButton} type="button" onClick={() => navigate('/auth/activate')}>
            账号激活
          </button>
        </span>
      </div>
    </div>
  )
}

export default LoginPage
