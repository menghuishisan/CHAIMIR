// PlatformLoginPage 平台管理员登录通道(仅 SaaS 形态注册路由,私有化部署下该路径不存在):
// 与学校用户入口彻底分离 —— 不做「选择角色」,不与手机号登录混排(规范/对齐清单 §认证)。
// 特权通道用落印语义强调(朱砂只用于品牌章与落印动作),其余仍是同一块墨色底层(FE-6),
// 不另造一套暗色皮肤。会话不提供「免登录」:超管令牌不落 localStorage,只随浏览器会话存续。

import { useCallback, useId, useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { ShieldCheck } from 'lucide-react'
import { Icon } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { loginEntryPath, persistLoginTokens } from '../../../../utils/authSession'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { passwordRequiredError, requiredError, useFieldErrors } from './auth-form'
import {
  AuthBrandMark,
  AuthFormError,
  AuthHeading,
  AuthPanel,
  AuthSubmit,
  AuthTextField,
} from './auth-ui'

/**
 * PlatformLoginPage 渲染账号密码表单并在成功后进入平台管理首个功能页。
 * 落点由服务端返回的角色决定(不接受任何客户端传入角色),改密要求同样由服务端裁决。
 */
export default function PlatformLoginPage() {
  const navigate = useNavigate()
  const fieldId = useId()

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const { errors, setError } = useFieldErrors()

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const usernameOk = setError('username', requiredError(username, '请输入平台管理账号'))
      const passwordOk = setError('password', passwordRequiredError(password, '请输入密码'))
      if (!usernameOk || !passwordOk) return

      setSubmitting(true)
      setFormError(null)
      try {
        const response = await api.identity.loginPlatform({ username: username.trim(), password, remember: false })
        persistLoginTokens(response)
        navigate(loginEntryPath(response), { replace: true })
      } catch (loginError) {
        setFormError(userFacingErrorMessage(loginError, '登录失败,请检查账号和密码后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [navigate, password, setError, username],
  )

  return (
    <AuthPanel>
      <div className="w-full max-w-sm">
        {/* 品牌行:朱砂章标明这是落印级别的特权入口,副标题点明通道用途 */}
        <AuthBrandMark large subtitle="平台管理通道" />

        <form className="mt-10" onSubmit={handleSubmit} noValidate>
          <AuthHeading title="平台管理员登录" description="仅限平台运营人员使用,操作全程记入审计" />

          <AuthTextField
            label="管理账号"
            id={`${fieldId}-username`}
            autoComplete="username"
            value={username}
            error={errors.username}
            onValueChange={setUsername}
            onBlur={() => setError('username', requiredError(username, '请输入平台管理账号'))}
          />

          <AuthTextField
            label="密码"
            id={`${fieldId}-password`}
            type="password"
            autoComplete="current-password"
            value={password}
            error={errors.password}
            onValueChange={setPassword}
            onBlur={() => setError('password', passwordRequiredError(password, '请输入密码'))}
          />

          <AuthFormError message={formError} />

          <AuthSubmit loading={submitting}>登录</AuthSubmit>

          <p className="mt-5 flex items-start gap-2 text-xs text-on-dark-sub">
            <Icon icon={ShieldCheck} size="sm" className="mt-px shrink-0" />
            本通道不用于学校师生登录;学校账号请从学校登录入口进入。
          </p>
        </form>
      </div>
    </AuthPanel>
  )
}
