// ForgotPasswordPage 找回密码(公共页,底层全裸露形态):
// 单页表单 —— 手机号 + 短信验证码 + 新密码,一次提交完成重置(POST auth/password/reset)。
// 学校由服务端按手机号归属判定:后端 resolveSMSCredentialTenant 在手机号只属于一所学校时
// 自动定位,一号多校时返回「请选择正确的学校」的用户向错误。因此本页不要求用户填写
// 任何内部标识(学校数字编号即租户主键,属内部字段,不进用户界面)。
// 重置成功后服务端会吊销该账号全部会话,故引导用户回登录页重新登录。

import React, { useCallback, useId, useState } from 'react'
import { SmsScene } from '@chaimir/api-client'
import { ArrowLeft } from 'lucide-react'
import { api } from '../../../../app/api'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import {
  confirmPasswordError,
  passwordRuleError,
  phoneError,
  requiredError,
  useFieldErrors,
  useSmsCooldown,
} from '../../authForm'
import {
  AuthFormError,
  AuthHeading,
  AuthPanel,
  AuthPrimaryLink,
  AuthQuietLink,
  AuthSmsField,
  AuthSubmit,
  AuthSuccess,
  AuthTextField,
} from '../../components/AuthUI'

/**
 * ForgotPasswordPage 处理短信验证码重置密码。
 */
export default function ForgotPasswordPage() {
  const fieldId = useId()
  const [phone, setPhone] = useState('')
  const [smsCode, setSmsCode] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const { errors, setError } = useFieldErrors()
  const [formError, setFormError] = useState<string | null>(null)
  const [sendingSms, setSendingSms] = useState(false)
  const { seconds: smsCooldown, start: startSmsCooldown } = useSmsCooldown()
  const [submitting, setSubmitting] = useState(false)
  const [done, setDone] = useState(false)

  /** handleSendSms 发送重置场景验证码并启动重发倒计时 */
  const handleSendSms = useCallback(async () => {
    if (!setError('phone', phoneError(phone, '手机号'))) return
    setSendingSms(true)
    setFormError(null)
    try {
      await api.identity.sendSMS({ phone: phone.trim(), scene: SmsScene.RESET })
      startSmsCooldown()
    } catch (sendError) {
      setFormError(userFacingErrorMessage(sendError, '验证码发送失败,请稍后重试。'))
    } finally {
      setSendingSms(false)
    }
  }, [phone, setError, startSmsCooldown])

  /** handleSubmit 提交重置;成功后引导重新登录(服务端已吊销该账号全部会话) */
  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const phoneOk = setError('phone', phoneError(phone, '手机号'))
      const codeOk = setError('smsCode', requiredError(smsCode, '请输入验证码'))
      const passwordOk = setError('new', passwordRuleError(newPassword, '新密码'))
      const confirmOk = setError('confirm', confirmPasswordError(confirmPassword, newPassword, '新密码'))
      if (!phoneOk || !codeOk || !passwordOk || !confirmOk) return

      setSubmitting(true)
      setFormError(null)
      try {
        await api.identity.resetPassword({
          phone: phone.trim(),
          code: smsCode.trim(),
          new_password: newPassword,
        })
        setDone(true)
      } catch (resetError) {
        setFormError(userFacingErrorMessage(resetError, '密码重置失败,请确认手机号与验证码后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [confirmPassword, newPassword, phone, setError, smsCode],
  )

  if (done) {
    return (
      <AuthPanel>
        <AuthSuccess
          title="新密码已生效"
          description="为保护账号安全,原有登录状态已全部退出,请使用新密码重新登录。"
        >
          <AuthPrimaryLink to="/auth/login">返回登录</AuthPrimaryLink>
        </AuthSuccess>
      </AuthPanel>
    )
  }

  return (
    <AuthPanel>
      <form className="w-full max-w-sm" onSubmit={handleSubmit} noValidate>
        <AuthHeading title="找回密码" description="通过账号绑定的手机号验证身份后设置新密码。" />

        <AuthTextField
          label="手机号"
          id={`${fieldId}-phone`}
          type="tel"
          autoComplete="tel"
          inputMode="numeric"
          value={phone}
          error={errors.phone}
          onValueChange={setPhone}
          onBlur={() => setError('phone', phoneError(phone, '手机号'))}
        />

        <AuthSmsField
          id={`${fieldId}-sms`}
          value={smsCode}
          error={errors.smsCode}
          onValueChange={setSmsCode}
          onBlur={() => setError('smsCode', requiredError(smsCode, '请输入验证码'))}
          cooldownSeconds={smsCooldown}
          sending={sendingSms}
          onSend={handleSendSms}
        />

        <AuthTextField
          label="新密码"
          id={`${fieldId}-new`}
          type="password"
          autoComplete="new-password"
          value={newPassword}
          error={errors.new}
          onValueChange={setNewPassword}
          onBlur={() => setError('new', passwordRuleError(newPassword, '新密码'))}
        />

        <AuthTextField
          label="确认新密码"
          id={`${fieldId}-confirm`}
          type="password"
          autoComplete="new-password"
          value={confirmPassword}
          error={errors.confirm}
          onValueChange={setConfirmPassword}
          onBlur={() => setError('confirm', confirmPasswordError(confirmPassword, newPassword, '新密码'))}
        />

        <AuthFormError message={formError} />

        <AuthSubmit loading={submitting}>确认重置</AuthSubmit>

        <AuthQuietLink to="/auth/login" icon={ArrowLeft}>
          返回登录
        </AuthQuietLink>
      </form>
    </AuthPanel>
  )
}
