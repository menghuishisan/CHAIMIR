// ForgotPasswordPage 通过短信验证重置密码，字段与 identity 后端契约一致。

import React, { useCallback, useState } from 'react'
import { SmsScene } from '@chaimir/api-client'
import { Button, Callout, FormField, Input } from '@chaimir/ui'
import { ArrowLeft } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../app/api'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import styles from './auth-form.module.css'

/**
 * ForgotPasswordPage 发送重置验证码并提交完整的单页重置表单。
 */
const ForgotPasswordPage: React.FC = () => {
  const navigate = useNavigate()
  const [tenantId, setTenantId] = useState('')
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [sendingSms, setSendingSms] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  /**
   * validTenantId 校验后端要求的学校数字编号。
   */
  const validTenantId = useCallback((): string | null => {
    if (!/^[1-9]\d*$/.test(tenantId)) {
      setError('请输入学校管理员提供的学校编号。')
      return null
    }
    return tenantId
  }, [tenantId])

  /**
   * handleSendSms 请求重置密码验证码。
   */
  const handleSendSms = useCallback(async () => {
    const parsedTenantId = validTenantId()
    if (!parsedTenantId || !phone.trim()) {
      if (parsedTenantId) {
        setError('请先输入绑定手机号。')
      }
      return
    }
    setSendingSms(true)
    setError(null)
    setMessage(null)
    try {
      await api.identity.sendSMS({ phone: phone.trim(), scene: SmsScene.RESET, tenant_id: parsedTenantId })
      setMessage('验证码已发送，请查看手机短信。')
    } catch (smsError) {
      setError(userFacingErrorMessage(smsError, '验证码发送失败，请稍后重试。'))
    } finally {
      setSendingSms(false)
    }
  }, [phone, validTenantId])

  /**
   * handleReset 校验密码规则后调用后端重置密码接口。
   */
  const handleReset = useCallback(async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const parsedTenantId = validTenantId()
    if (!parsedTenantId) {
      return
    }
    if (!phone.trim() || !code.trim() || !password || !confirmPassword) {
      setError('请完整填写手机号、验证码和新密码。')
      return
    }
    if (password.length < 8 || !/[A-Za-z]/.test(password) || !/\d/.test(password)) {
      setError('新密码至少 8 位，并同时包含字母和数字。')
      return
    }
    if (password !== confirmPassword) {
      setError('两次输入的新密码不一致，请重新确认。')
      return
    }

    setSubmitting(true)
    setError(null)
    setMessage(null)
    try {
      await api.identity.resetPassword({
        phone: phone.trim(),
        code: code.trim(),
        new_password: password,
        tenant_id: parsedTenantId,
      })
      setMessage('密码已重置，请返回登录页使用新密码登录。')
    } catch (resetError) {
      setError(userFacingErrorMessage(resetError, '密码重置失败，请检查验证码后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [code, confirmPassword, password, phone, validTenantId])

  return (
    <form className={styles.form} onSubmit={handleReset}>
      <div>
        <h1 className={styles.title}>找回密码</h1>
        <p className={styles.description}>通过学校编号、绑定手机号和短信验证码重置密码。</p>
      </div>

      {error && <div className={styles.error} role="alert">{error}</div>}
      {message && <Callout variant="success" title="操作完成">{message}</Callout>}

      <div className={styles.fields}>
        <FormField label="学校编号" htmlFor="reset-tenant-id" helperText="由学校管理员提供" required>
          <Input id="reset-tenant-id" fullWidth inputMode="numeric" value={tenantId} onChange={(event) => setTenantId(event.target.value)} />
        </FormField>
        <FormField label="绑定手机号" htmlFor="reset-phone" required>
          <Input id="reset-phone" fullWidth inputMode="tel" autoComplete="tel" value={phone} onChange={(event) => setPhone(event.target.value)} />
        </FormField>
        <div className={styles.inline}>
          <FormField label="短信验证码" htmlFor="reset-code" required>
            <Input id="reset-code" fullWidth autoCapitalize="characters" autoComplete="one-time-code" value={code} onChange={(event) => setCode(event.target.value)} />
          </FormField>
          <Button className={styles.inlineButton} variant="outline" loading={sendingSms} onClick={handleSendSms}>获取验证码</Button>
        </div>
        <FormField label="新密码" htmlFor="reset-password" helperText="至少 8 位，并同时包含字母和数字" required>
          <Input id="reset-password" fullWidth type="password" autoComplete="new-password" value={password} onChange={(event) => setPassword(event.target.value)} />
        </FormField>
        <FormField label="确认新密码" htmlFor="reset-confirm-password" required>
          <Input id="reset-confirm-password" fullWidth type="password" autoComplete="new-password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} />
        </FormField>
        <Button block type="submit" loading={submitting}>确认重置</Button>
      </div>

      <button className={styles.linkButton} type="button" onClick={() => navigate('/auth/login')}>
        <ArrowLeft size={16} /> 返回登录
      </button>
    </form>
  )
}

export default ForgotPasswordPage
