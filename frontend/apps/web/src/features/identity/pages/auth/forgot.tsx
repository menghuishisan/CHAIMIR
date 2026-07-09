// ForgotPasswordPage 处理短信找回密码，按 identity 后端租户编号契约重置密码。

import React, { useCallback, useState } from 'react'
import type { ApiError } from '@chaimir/api-client'
import { SmsScene } from '@chaimir/api-client'
import { Button, Callout, Input } from '@chaimir/ui'
import { ArrowLeft } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../app/api'
import styles from './auth-form.module.css'

const ForgotPasswordPage: React.FC = () => {
  const navigate = useNavigate()
  const [step, setStep] = useState(1)
  const [tenantId, setTenantId] = useState('')
  const [phone, setPhone] = useState('')
  const [code, setCode] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [sendingSms, setSendingSms] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const parsedTenantId = Number(tenantId)

  /**
   * validateTenantId 确保重置密码请求带上后端需要的租户编号。
   */
  const validateTenantId = useCallback((): number | null => {
    if (!Number.isInteger(parsedTenantId) || parsedTenantId <= 0) {
      setError('请输入学校管理员提供的学校编号。')
      return null
    }
    return parsedTenantId
  }, [parsedTenantId])

  /**
   * handleSendSms 请求重置密码验证码。
   */
  const handleSendSms = useCallback(async () => {
    const validTenantId = validateTenantId()
    if (!validTenantId) {
      return
    }
    setSendingSms(true)
    setError(null)
    setMessage(null)
    try {
      await api.identity.sendSMS({ phone, scene: SmsScene.RESET, tenant_id: validTenantId })
      setMessage('验证码已发送，请查看手机短信。')
    } catch (smsError) {
      setError((smsError as ApiError).message || '验证码发送失败，请稍后重试。')
    } finally {
      setSendingSms(false)
    }
  }, [phone, validateTenantId])

  /**
   * handleContinue 校验第一步必要信息后进入密码设置。
   */
  const handleContinue = useCallback(() => {
    const validTenantId = validateTenantId()
    if (!validTenantId) {
      return
    }
    if (!phone.trim() || !code.trim()) {
      setError('请输入手机号和短信验证码。')
      return
    }
    setError(null)
    setMessage(null)
    setStep(2)
  }, [code, phone, validateTenantId])

  /**
   * handleReset 调用后端重置密码接口。
   */
  const handleReset = useCallback(async () => {
    const validTenantId = validateTenantId()
    if (!validTenantId) {
      return
    }
    if (!password || password !== confirmPassword) {
      setError('两次输入的密码不一致，请重新确认。')
      return
    }
    setSubmitting(true)
    setError(null)
    setMessage(null)
    try {
      await api.identity.resetPassword({
        phone,
        code,
        new_password: password,
        tenant_id: validTenantId,
      })
      setMessage('密码已重置，请返回登录页使用新密码登录。')
    } catch (resetError) {
      setError((resetError as ApiError).message || '密码重置失败，请检查验证码后重试。')
    } finally {
      setSubmitting(false)
    }
  }, [code, confirmPassword, password, phone, validateTenantId])

  return (
    <div className={styles.form}>
      <div>
        <h2 className={styles.title}>找回密码</h2>
        <p className={styles.description}>通过学校编号、手机号和短信验证码重置登录密码。</p>
      </div>

      <div className={styles.progress} aria-label="找回密码进度">
        <span className={styles.progressActive} />
        <span className={step >= 2 ? styles.progressActive : styles.progressIdle} />
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && (
        <Callout variant="success" title="操作已提交">
          {message}
        </Callout>
      )}

      <div className={styles.fields}>
        {step === 1 && (
          <>
            <Input fullWidth placeholder="学校编号" value={tenantId} onChange={(event) => setTenantId(event.target.value)} />
            <Input fullWidth placeholder="绑定手机号" value={phone} onChange={(event) => setPhone(event.target.value)} />
            <div className={styles.inline}>
              <Input fullWidth placeholder="验证码" value={code} onChange={(event) => setCode(event.target.value)} />
              <Button variant="outline" loading={sendingSms} onClick={handleSendSms}>
                获取验证码
              </Button>
            </div>
            <Button block onClick={handleContinue}>
              下一步
            </Button>
          </>
        )}

        {step === 2 && (
          <>
            <Input fullWidth type="password" placeholder="设置新密码" value={password} onChange={(event) => setPassword(event.target.value)} />
            <Input fullWidth type="password" placeholder="确认新密码" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} />
            <Button block loading={submitting} onClick={handleReset}>
              确认重置
            </Button>
          </>
        )}
      </div>

      <div className={styles.footerLinks}>
        <button className={styles.linkButton} type="button" onClick={() => navigate('/auth/login')}>
          <ArrowLeft size={16} /> 返回登录
        </button>
      </div>
    </div>
  )
}

export default ForgotPasswordPage
