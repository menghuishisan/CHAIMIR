// ActivatePage 账号激活(公共页,底层全裸露形态):
// 学校管理员下发激活码,首次使用者凭激活码设置自己的密码(POST auth/activate)。
// 激活码格式与有效期由服务端校验(明文只进 service 比对,不落库),前端不复刻校验规则,
// 只做「必填 + 密码强度」的就近校验。服务端激活接口不签发会话,成功后引导回登录页。

import React, { useCallback, useId, useState } from 'react'
import { ArrowLeft } from 'lucide-react'
import { api } from '../../../../app/api'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { confirmPasswordError, passwordRuleError, requiredError, useFieldErrors } from '../../authForm'
import {
  AuthFormError,
  AuthHeading,
  AuthPanel,
  AuthPrimaryLink,
  AuthQuietLink,
  AuthSubmit,
  AuthSuccess,
  AuthTextField,
} from '../../components/AuthUI'

/**
 * ActivatePage 处理激活码开通账号。
 */
export default function ActivatePage() {
  const fieldId = useId()
  const [activationCode, setActivationCode] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const { errors, setError } = useFieldErrors()
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [done, setDone] = useState(false)

  /** handleSubmit 提交激活;成功后引导登录(激活接口不签发会话) */
  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const codeOk = setError('code', requiredError(activationCode, '请输入激活码'))
      const passwordOk = setError('password', passwordRuleError(password, '密码'))
      const confirmOk = setError('confirm', confirmPasswordError(confirmPassword, password, '密码'))
      if (!codeOk || !passwordOk || !confirmOk) return

      setSubmitting(true)
      setFormError(null)
      try {
        await api.identity.activate({ activation_code: activationCode.trim(), password })
        setDone(true)
      } catch (activateError) {
        setFormError(userFacingErrorMessage(activateError, '激活失败,请确认激活码是否正确且仍在有效期内。'))
      } finally {
        setSubmitting(false)
      }
    },
    [activationCode, confirmPassword, password, setError],
  )

  if (done) {
    return (
      <AuthPanel>
        <AuthSuccess title="账号已激活" description="现在可以用刚设置的密码登录平台。">
          <AuthPrimaryLink to="/auth/login">前往登录</AuthPrimaryLink>
        </AuthSuccess>
      </AuthPanel>
    )
  }

  return (
    <AuthPanel>
      <form className="w-full max-w-sm" onSubmit={handleSubmit} noValidate>
        <AuthHeading title="激活账号" description="使用学校下发的激活码开通账号并设置密码。" />

        <AuthTextField
          label="激活码"
          id={`${fieldId}-code`}
          autoComplete="one-time-code"
          value={activationCode}
          error={errors.code}
          onValueChange={setActivationCode}
          onBlur={() => setError('code', requiredError(activationCode, '请输入激活码'))}
        />

        <AuthTextField
          label="设置密码"
          id={`${fieldId}-password`}
          type="password"
          autoComplete="new-password"
          value={password}
          error={errors.password}
          onValueChange={setPassword}
          onBlur={() => setError('password', passwordRuleError(password, '密码'))}
        />

        <AuthTextField
          label="确认密码"
          id={`${fieldId}-confirm`}
          type="password"
          autoComplete="new-password"
          value={confirmPassword}
          error={errors.confirm}
          onValueChange={setConfirmPassword}
          onBlur={() => setError('confirm', confirmPasswordError(confirmPassword, password, '密码'))}
        />

        <AuthFormError message={formError} />

        <AuthSubmit loading={submitting}>立即激活</AuthSubmit>

        <AuthQuietLink to="/auth/login" icon={ArrowLeft}>
          返回登录
        </AuthQuietLink>
      </form>
    </AuthPanel>
  )
}
