// ActivatePage 处理首次账号激活，直接提交 identity 后端激活接口。

import React, { useCallback, useState } from 'react'
import { Button, Callout, FormField, Input } from '@chaimir/ui'
import { ArrowLeft } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../app/api'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import styles from './auth-form.module.css'

const ActivatePage: React.FC = () => {
  const navigate = useNavigate()
  const [activationCode, setActivationCode] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  /**
   * handleActivate 校验两次密码一致后调用后端激活账号。
   */
  const handleActivate = useCallback(async () => {
    setMessage(null)
    setError(null)
    if (!activationCode.trim()) {
      setError('请输入管理员下发的激活码。')
      return
    }
    if (!password || password !== confirmPassword) {
      setError('两次输入的密码不一致，请重新确认。')
      return
    }
    if (password.length < 8 || !/[A-Za-z]/.test(password) || !/\d/.test(password)) {
      setError('新密码至少 8 位，并同时包含字母和数字。')
      return
    }

    setSubmitting(true)
    try {
      await api.identity.activate({
        activation_code: activationCode.trim(),
        password,
      })
      setMessage('账号已激活，请返回登录页使用新密码登录。')
    } catch (activateError) {
      setError(userFacingErrorMessage(activateError, '账号激活失败，请检查激活码后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [activationCode, confirmPassword, password])

  return (
    <form className={styles.form} onSubmit={(event) => {
      event.preventDefault()
      void handleActivate()
    }}>
      <div>
        <h1 className={styles.title}>激活账号</h1>
        <p className={styles.description}>首次登录请使用管理员下发的激活码完成账号激活。</p>
      </div>

      {error && <div className={styles.error} role="alert">{error}</div>}
      {message && (
        <Callout variant="success" title="激活完成">
          {message}
        </Callout>
      )}

      <div className={styles.fields}>
        <FormField label="激活码" htmlFor="activation-code" required>
          <Input id="activation-code" fullWidth autoComplete="one-time-code" value={activationCode} onChange={(event) => setActivationCode(event.target.value)} />
        </FormField>
        <FormField label="新密码" htmlFor="activation-password" helperText="至少 8 位，并同时包含字母和数字" required>
          <Input id="activation-password" fullWidth type="password" autoComplete="new-password" value={password} onChange={(event) => setPassword(event.target.value)} />
        </FormField>
        <FormField label="确认新密码" htmlFor="activation-confirm-password" required>
          <Input id="activation-confirm-password" fullWidth type="password" autoComplete="new-password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} />
        </FormField>
        <Button block type="submit" loading={submitting}>
          立即激活
        </Button>
      </div>

      <div className={styles.footerLinks}>
        <button className={styles.linkButton} type="button" onClick={() => navigate('/auth/login')}>
          <ArrowLeft size={16} /> 返回登录
        </button>
      </div>
    </form>
  )
}

export default ActivatePage
