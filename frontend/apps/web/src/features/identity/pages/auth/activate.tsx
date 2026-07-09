// ActivatePage 处理首次账号激活，直接提交 identity 后端激活接口。

import React, { useCallback, useState } from 'react'
import type { ApiError } from '@chaimir/api-client'
import { Button, Callout, Input } from '@chaimir/ui'
import { ArrowLeft } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../app/api'
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

    setSubmitting(true)
    try {
      await api.identity.activate({
        activation_code: activationCode.trim(),
        password,
      })
      setMessage('账号已激活，请返回登录页使用新密码登录。')
    } catch (activateError) {
      setError((activateError as ApiError).message || '账号激活失败，请检查激活码后重试。')
    } finally {
      setSubmitting(false)
    }
  }, [activationCode, confirmPassword, password])

  return (
    <div className={styles.form}>
      <div>
        <h2 className={styles.title}>激活账号</h2>
        <p className={styles.description}>首次登录请使用管理员下发的激活码完成账号激活。</p>
      </div>

      {error && <div className={styles.error}>{error}</div>}
      {message && (
        <Callout variant="success" title="激活完成">
          {message}
        </Callout>
      )}

      <div className={styles.fields}>
        <Input
          fullWidth
          placeholder="激活码"
          value={activationCode}
          onChange={(event) => setActivationCode(event.target.value)}
        />
        <Input
          fullWidth
          type="password"
          placeholder="设置新密码"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
        />
        <Input
          fullWidth
          type="password"
          placeholder="确认新密码"
          value={confirmPassword}
          onChange={(event) => setConfirmPassword(event.target.value)}
        />
        <Button block loading={submitting} onClick={handleActivate}>
          立即激活
        </Button>
      </div>

      <div className={styles.footerLinks}>
        <button className={styles.linkButton} type="button" onClick={() => navigate('/auth/login')}>
          <ArrowLeft size={16} /> 返回登录
        </button>
      </div>
    </div>
  )
}

export default ActivatePage
