// ChangePasswordPage 强制改密拦截页:服务端标记 must_change_pwd 的账号
// 必须先完成改密才能进入功能区(RoleGuard 会把此类会话统一拦到本页)。
// 校验在 blur 就近进行;完成后跳转登录时暂存的角色入口。

import React, { useCallback, useId, useState } from 'react'
import { useNavigate } from 'react-router'
import { api } from '../../../../app/api'
import {
  clearLoginTokens,
  completeRequiredPasswordChange,
} from '../../../../utils/authSession'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import {
  confirmPasswordError,
  passwordRequiredError,
  passwordRuleError,
  useFieldErrors,
} from '../../authForm'
import {
  AuthFormError,
  AuthHeading,
  AuthPanel,
  AuthQuietAction,
  AuthSubmit,
  AuthTextField,
} from '../../components/AuthUI'

/**
 * newPasswordError 校验新密码:强度规则之外,还要求与当前密码不同 ——
 * 强制改密的目的就是换掉已知的初始密码,原样重填等于没改。
 */
function newPasswordError(newPassword: string, oldPassword: string): string | null {
  const ruleError = passwordRuleError(newPassword, '新密码')
  if (ruleError) return ruleError
  return newPassword === oldPassword ? '新密码不能与当前密码相同' : null
}

/**
 * ChangePasswordPage 处理首次登录/重置后的强制改密。
 */
export default function ChangePasswordPage() {
  const navigate = useNavigate()
  const fieldId = useId()
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const { errors, setError } = useFieldErrors()
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  /** handleSubmit 提交改密;成功后进入登录时暂存的角色入口 */
  const handleSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const oldOk = setError('old', passwordRequiredError(oldPassword, '请输入当前密码'))
      const newOk = setError('new', newPasswordError(newPassword, oldPassword))
      const confirmOk = setError('confirm', confirmPasswordError(confirmPassword, newPassword, '新密码'))
      if (!oldOk || !newOk || !confirmOk) return

      setSubmitting(true)
      setFormError(null)
      try {
        await api.identity.changePassword({ old_password: oldPassword, new_password: newPassword })
        navigate(completeRequiredPasswordChange(), { replace: true })
      } catch (changeError) {
        setFormError(userFacingErrorMessage(changeError, '密码修改失败,请检查当前密码后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [confirmPassword, navigate, newPassword, oldPassword, setError],
  )

  /** handleLogout 尽力吊销服务端会话,并始终清除本地凭证返回登录页 */
  const handleLogout = useCallback(async () => {
    try {
      await api.identity.logout()
    } catch {
      // 网络失败也必须允许用户离开:本地凭证已清除,服务端令牌按有效期自然过期
    } finally {
      clearLoginTokens()
      navigate('/auth/login', { replace: true })
    }
  }, [navigate])

  return (
    <AuthPanel>
      <form className="w-full max-w-sm" onSubmit={handleSubmit} noValidate>
        <AuthHeading title="请先修改初始密码" description="完成密码修改后才能继续使用平台。" />

        <AuthTextField
          label="当前密码"
          id={`${fieldId}-old`}
          type="password"
          autoComplete="current-password"
          value={oldPassword}
          error={errors.old}
          onValueChange={setOldPassword}
          onBlur={() => setError('old', passwordRequiredError(oldPassword, '请输入当前密码'))}
        />

        <AuthTextField
          label="新密码"
          id={`${fieldId}-new`}
          type="password"
          autoComplete="new-password"
          value={newPassword}
          error={errors.new}
          onValueChange={setNewPassword}
          onBlur={() => setError('new', newPasswordError(newPassword, oldPassword))}
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

        <AuthSubmit loading={submitting}>确认修改</AuthSubmit>

        <AuthQuietAction onClick={handleLogout}>退出登录</AuthQuietAction>
      </form>
    </AuthPanel>
  )
}
