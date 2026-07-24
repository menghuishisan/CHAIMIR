// ChangePasswordPage 处理首次登录强制改密，只调用后端允许的个人密码接口。

import React, { useCallback, useState } from 'react'
import { Button, FormField, Input } from '@chaimir/ui'
import { LogOut } from 'lucide-react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../../app/api'
import {
  clearLoginTokens,
  completeRequiredPasswordChange,
} from '../../../../utils/authSession'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import styles from './auth-form.module.css'

/**
 * ChangePasswordPage 校验新密码并在成功后放行到原角色入口。
 */
const ChangePasswordPage: React.FC = () => {
  const navigate = useNavigate()
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  /**
   * handleSubmit 调用服务端改密接口，成功后解除当前会话的强制拦截。
   */
  const handleSubmit = useCallback(async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    setError(null)
    if (!oldPassword || !newPassword || !confirmPassword) {
      setError('请完整填写当前密码、新密码和确认密码。')
      return
    }
    if (newPassword.length < 8 || !/[A-Za-z]/.test(newPassword) || !/\d/.test(newPassword)) {
      setError('新密码至少 8 位，并同时包含字母和数字。')
      return
    }
    if (newPassword !== confirmPassword) {
      setError('两次输入的新密码不一致，请重新确认。')
      return
    }
    if (newPassword === oldPassword) {
      setError('新密码不能与当前密码相同。')
      return
    }

    setSubmitting(true)
    try {
      await api.identity.changePassword({ old_password: oldPassword, new_password: newPassword })
      navigate(completeRequiredPasswordChange(), { replace: true })
    } catch (changeError) {
      setError(userFacingErrorMessage(changeError, '密码修改失败，请检查当前密码后重试。'))
    } finally {
      setSubmitting(false)
    }
  }, [confirmPassword, navigate, newPassword, oldPassword])

  /**
   * handleLogout 尽力吊销服务端会话，并始终清除本地凭证返回登录页。
   */
  const handleLogout = useCallback(async () => {
    try {
      await api.identity.logout()
    } catch {
      // 密码已修改时必须清除本地令牌，即使服务端退出请求未完成。
    } finally {
      clearLoginTokens()
      navigate('/auth/login', { replace: true })
    }
  }, [navigate])

  return (
    <form className={styles.form} onSubmit={handleSubmit}>
      <div>
        <h2 className={styles.title}>请先修改初始密码</h2>
        <p className={styles.description}>完成密码修改后才能继续使用平台功能。</p>
      </div>

      {error && <div className={styles.error} role="alert">{error}</div>}

      <div className={styles.fields}>
        <FormField label="当前密码" htmlFor="current-password" required>
          <Input id="current-password" fullWidth type="password" autoComplete="current-password" value={oldPassword} onChange={(event) => setOldPassword(event.target.value)} />
        </FormField>
        <FormField label="新密码" htmlFor="new-password" helperText="至少 8 位，并同时包含字母和数字" required>
          <Input id="new-password" fullWidth type="password" autoComplete="new-password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} />
        </FormField>
        <FormField label="确认新密码" htmlFor="confirm-password" required>
          <Input id="confirm-password" fullWidth type="password" autoComplete="new-password" value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} />
        </FormField>
        <Button block type="submit" loading={submitting}>完成修改并进入系统</Button>
        <Button block variant="ghost" icon={<LogOut size={16} />} onClick={() => void handleLogout()}>退出并重新登录</Button>
      </div>
    </form>
  )
}

export default ChangePasswordPage
