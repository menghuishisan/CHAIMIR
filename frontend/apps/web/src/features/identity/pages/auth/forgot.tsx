// ForgotPasswordPage 找回密码(公共页,底层全裸露形态):
// 两步(设计 §7.1)—— ① 手机号 + 短信验证码 → ② 新密码 + 实时规则清单,最后一次提交完成重置(POST auth/password/reset)。
// 验证码是否有效只由服务端在提交时裁决,第一步只做「填没填、格式对不对」的就近校验;
// 第二步可以返回修改手机号,已填内容都留在内存里,验证码不进地址与本地存储(CLAUDE.md §9)。
// 学校由服务端按手机号归属判定:一号多校时返回「请选择正确的学校」的用户向错误,
// 本页不要求用户填写任何内部标识。重置成功后服务端会吊销该账号全部会话,故引导回登录页。

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
  useSmsCooldown,
} from '../../authForm'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import {
  AuthFormBrand,
  AuthFormError,
  AuthHeading,
  AuthPanel,
  AuthPrimaryLink,
  AuthQuietAction,
  AuthQuietLink,
  AuthSmsField,
  AuthSteps,
  AuthSubmit,
  AuthSuccess,
  AuthTextField,
  PasswordRuleChecklist,
} from '../../components/AuthUI'

/** 两步的名称,顺序即步骤顺序 */
const STEPS = ['验证手机号', '设置新密码']

/**
 * ForgotPasswordPage 处理短信验证码重置密码。
 */
export default function ForgotPasswordPage() {
  const fieldId = useId()
  const [step, setStep] = useState<0 | 1>(0)
  const [phone, setPhone] = useState('')
  const [smsCode, setSmsCode] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const { errors, validate } = useFormValidation(
    () => ({
      phone: phoneError(phone, '手机号'),
      smsCode: requiredError(smsCode, '请输入验证码'),
      new: passwordRuleError(newPassword, '新密码'),
      confirm: confirmPasswordError(confirmPassword, newPassword, '新密码'),
    }),
    step,
  )
  const [formError, setFormError] = useState<string | null>(null)
  const [sendingSms, setSendingSms] = useState(false)
  const { seconds: smsCooldown, start: startSmsCooldown } = useSmsCooldown()
  const [submitting, setSubmitting] = useState(false)
  const [done, setDone] = useState(false)

  /** handleSendSms 发送重置场景验证码并启动重发倒计时 */
  const handleSendSms = useCallback(async () => {
    if (!validate(['phone'])) return
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
  }, [phone, startSmsCooldown, validate])

  /** handleNext 第一步只校验手机号与验证码是否填写,通过后进入设置新密码 */
  const handleNext = useCallback(
    (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate(['phone', 'smsCode'])) return
      setFormError(null)
      setStep(1)
    },
    [validate],
  )

  /** handleSubmit 提交重置;成功后引导重新登录(服务端已吊销该账号全部会话) */
  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      if (!validate(['new', 'confirm'])) return

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
        setFormError(userFacingErrorMessage(resetError, '密码重置失败,请返回上一步确认手机号与验证码后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [newPassword, phone, smsCode, validate],
  )

  if (done) {
    return (
      <AuthPanel>
        <AuthSuccess
          title="新密码已生效"
          description="为保护账号安全,其他设备上的登录已全部退出,请用新密码重新登录。"
        >
          <AuthPrimaryLink to="/auth/login">去登录</AuthPrimaryLink>
        </AuthSuccess>
      </AuthPanel>
    )
  }

  return (
    <AuthPanel>
      <AuthFormBrand />
      <div className="w-full max-w-sm">
        <AuthHeading title="找回密码" description="用账号绑定的手机号验证身份,然后设置新密码。" />
        <AuthSteps steps={STEPS} current={step} />

        {step === 0 ? (
          <form onSubmit={handleNext} noValidate>
            <AuthTextField
              label="手机号"
              id={`${fieldId}-phone`}
              type="tel"
              autoComplete="tel"
              inputMode="numeric"
              value={phone}
              error={errors.phone}
              onValueChange={setPhone}
              onBlur={() => validate(['phone'])}
            />

            <AuthSmsField
              id={`${fieldId}-sms`}
              value={smsCode}
              error={errors.smsCode}
              onValueChange={setSmsCode}
              onBlur={() => validate(['smsCode'])}
              cooldownSeconds={smsCooldown}
              sending={sendingSms}
              onSend={handleSendSms}
            />

            <AuthFormError message={formError} />

            <AuthSubmit>下一步</AuthSubmit>

            <AuthQuietLink to="/auth/login" icon={ArrowLeft}>
              返回登录
            </AuthQuietLink>
          </form>
        ) : (
          <form onSubmit={handleSubmit} noValidate>
            <p className="mt-6 text-sm text-on-dark-sub">
              为手机号 <span className="font-mono text-on-dark">{phone.trim()}</span> 的账号设置新密码
            </p>

            <AuthTextField
              label="新密码"
              id={`${fieldId}-new`}
              type="password"
              autoComplete="new-password"
              value={newPassword}
              error={errors.new}
              onValueChange={setNewPassword}
              onBlur={() => validate(['new'])}
            />
            <PasswordRuleChecklist value={newPassword} />

            <AuthTextField
              label="确认新密码"
              id={`${fieldId}-confirm`}
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              error={errors.confirm}
              onValueChange={setConfirmPassword}
              onBlur={() => validate(['confirm'])}
            />

            <AuthFormError message={formError} />

            <AuthSubmit loading={submitting}>确认重置</AuthSubmit>

            <AuthQuietAction
              icon={ArrowLeft}
              onClick={() => {
                setFormError(null)
                setStep(0)
              }}
            >
              返回修改手机号
            </AuthQuietAction>
          </form>
        )}
      </div>
    </AuthPanel>
  )
}
