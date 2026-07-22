// SecuritySettings 管理当前账号的密码、绑定手机号和登录令牌刷新操作。

import React, { useCallback, useState } from 'react'
import { SmsScene } from '@chaimir/api-client'
import { Button, Callout, FormField, Input } from '@chaimir/ui'
import { KeyRound, RefreshCw, Smartphone } from 'lucide-react'
import { api } from '../../../../../app/api'
import { invalidateAppResource } from '../../../../../app/resourceInvalidation'
import {
  getStoredRefreshToken,
  persistRefreshedTokens,
} from '../../../../../utils/authSession'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import styles from '../../identity-admin.module.css'

/**
 * SecuritySettings 通过 identity 公开接口完成账号安全设置，不在浏览器伪造结果。
 */
export function SecuritySettings(): React.ReactElement {
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [phone, setPhone] = useState('')
  const [smsCode, setSmsCode] = useState('')
  const [pendingAction, setPendingAction] = useState<'password' | 'sms' | 'phone' | 'refresh' | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  /** runAction 统一处理安全操作的加载和用户向反馈。 */
  const runAction = useCallback(async (
    action: Exclude<typeof pendingAction, null>,
    task: () => Promise<void>,
    success: string,
  ) => {
    setPendingAction(action)
    setMessage(null)
    setError(null)
    try {
      await task()
      setMessage(success)
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '操作未完成，请检查输入后重试。'))
    } finally {
      setPendingAction(null)
    }
  }, [])

  /** handleChangePassword 校验并提交当前账号密码变更。 */
  const handleChangePassword = useCallback(() => {
    if (!oldPassword || newPassword.length < 8 || !/[A-Za-z]/.test(newPassword) || !/\d/.test(newPassword)) {
      setError('请填写当前密码，新密码至少 8 位并同时包含字母和数字。')
      return
    }
    void runAction('password', async () => {
      await api.identity.changePassword({ old_password: oldPassword, new_password: newPassword })
      setOldPassword('')
      setNewPassword('')
    }, '密码已修改。')
  }, [newPassword, oldPassword, runAction])

  /** handleSendPhoneCode 发送变更绑定手机号所需验证码。 */
  const handleSendPhoneCode = useCallback(() => {
    if (!/^1\d{10}$/.test(phone.trim())) {
      setError('请输入正确的 11 位手机号。')
      return
    }
    void runAction('sms', () => api.identity.sendSMS({
      phone: phone.trim(),
      scene: SmsScene.CHANGE_PHONE,
    }), '验证码已发送，请查看手机短信。')
  }, [phone, runAction])

  /** handleChangePhone 提交手机号和验证码并以服务端结果为准。 */
  const handleChangePhone = useCallback(() => {
    if (!/^1\d{10}$/.test(phone.trim()) || !smsCode.trim()) {
      setError('请填写正确的手机号和短信验证码。')
      return
    }
    void runAction('phone', async () => {
      await api.identity.changePhone({ phone: phone.trim(), code: smsCode.trim() })
      invalidateAppResource('profile')
      setPhone('')
      setSmsCode('')
    }, '绑定手机号已更新。')
  }, [phone, runAction, smsCode])

  /** handleRefreshSession 使用服务端刷新令牌替换当前登录凭证。 */
  const handleRefreshSession = useCallback(() => {
    const refreshToken = getStoredRefreshToken()
    if (!refreshToken) {
      setError('当前会话没有可用的刷新凭证，请重新登录。')
      return
    }
    void runAction('refresh', async () => {
      const response = await api.identity.refreshToken({ refresh_token: refreshToken })
      if (!response.access_token || !response.refresh_token) {
        throw new Error('登录状态刷新响应不完整')
      }
      persistRefreshedTokens({ access_token: response.access_token, refresh_token: response.refresh_token, must_change_pwd: response.must_change_pwd })
    }, '登录状态已刷新。')
  }, [runAction])

  return (
    <div className={styles.grid}>
      {error && <div className={`${styles.error} ${styles.wide}`} role="alert">{error}</div>}
      {message && <Callout className={styles.wide} variant="success" title="操作完成">{message}</Callout>}

      <section className={styles.panel}>
        <h2><KeyRound size={18} /> 修改密码</h2>
        <FormField label="当前密码" htmlFor="profile-current-password" required>
          <Input id="profile-current-password" fullWidth type="password" autoComplete="current-password" value={oldPassword} onChange={(event) => setOldPassword(event.target.value)} />
        </FormField>
        <FormField label="新密码" htmlFor="profile-new-password" helperText="至少 8 位，并同时包含字母和数字" required>
          <Input id="profile-new-password" fullWidth type="password" autoComplete="new-password" value={newPassword} onChange={(event) => setNewPassword(event.target.value)} />
        </FormField>
        <Button icon={<KeyRound size={16} />} loading={pendingAction === 'password'} onClick={handleChangePassword}>保存密码</Button>
      </section>

      <section className={styles.panel}>
        <h2><Smartphone size={18} /> 更换手机号</h2>
        <FormField label="新手机号" htmlFor="profile-phone" required>
          <Input id="profile-phone" fullWidth inputMode="tel" autoComplete="tel" value={phone} onChange={(event) => setPhone(event.target.value)} />
        </FormField>
        <FormField label="短信验证码" htmlFor="profile-phone-code" required>
          <Input id="profile-phone-code" fullWidth autoCapitalize="characters" autoComplete="one-time-code" value={smsCode} onChange={(event) => setSmsCode(event.target.value)} />
        </FormField>
        <div className={styles.actions}>
          <Button variant="outline" loading={pendingAction === 'sms'} onClick={handleSendPhoneCode}>获取验证码</Button>
          <Button icon={<Smartphone size={16} />} loading={pendingAction === 'phone'} onClick={handleChangePhone}>确认更换</Button>
        </div>
      </section>

      <section className={`${styles.panel} ${styles.wide}`}>
        <h2>登录状态</h2>
        <p className={styles.muted}>重新向服务端换取登录凭证，不会修改账号资料。</p>
        <Button variant="outline" icon={<RefreshCw size={16} />} loading={pendingAction === 'refresh'} onClick={handleRefreshSession}>刷新登录状态</Button>
      </section>
    </div>
  )
}
