// SsoCallbackPage 统一认证中转页(公共页,路径 /auth/sso/:tenantCode):
// 一页承担两件事 ——
//   1) 从学校统一认证跳回时(URL 带 ticket):自动向后端验票换取会话并直达角色首页;
//   2) 直接进入本页时:提供 CAS 跳转入口与学校目录账号(LDAP)登录两种方式。
// service 参数固定为本页自身地址(去掉 ticket 查询串),后端按来源白名单校验,
// 前端不拼接任何外部地址,避免开放重定向。

import React, { useCallback, useEffect, useId, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { Button, Icon } from '@chaimir/ui'
import { ArrowLeft, LoaderCircle } from 'lucide-react'
import { api } from '../../../../app/api'
import { loginEntryPath, persistLoginTokens } from '../../../../utils/authSession'
import { safeExternalHttpUrl } from '../../../../utils/safeNavigation'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { passwordRequiredError, requiredError, useFieldErrors } from './auth-form'
import {
  AuthFormError,
  AuthHeading,
  AuthPanel,
  AuthQuietLink,
  AuthTextField,
} from './auth-ui'

/** ssoServiceUrl 返回本页自身地址(不含票据参数),作为 CAS 的 service 回跳目标 */
function ssoServiceUrl(tenantCode: string): string {
  return `${window.location.origin}/auth/sso/${encodeURIComponent(tenantCode)}`
}

/**
 * SsoCallbackPage 处理统一认证回跳与目录账号登录。
 */
export default function SsoCallbackPage() {
  const navigate = useNavigate()
  const fieldId = useId()
  const { tenantCode = '' } = useParams<{ tenantCode: string }>()
  const [searchParams] = useSearchParams()
  const ticket = searchParams.get('ticket')

  const [verifying, setVerifying] = useState(Boolean(ticket))
  const [redirecting, setRedirecting] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const { errors, setError } = useFieldErrors()
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  // 票据一次性:严格模式下 effect 会执行两次,重复验票会因票据已消耗而失败
  const verifiedTicket = useRef<string | null>(null)

  // 带票据进入即自动验票换会话
  useEffect(() => {
    if (!ticket || !tenantCode) return
    if (verifiedTicket.current === ticket) return
    verifiedTicket.current = ticket
    let active = true
    setVerifying(true)
    api.identity
      .casCallback(tenantCode, { ticket, service: ssoServiceUrl(tenantCode) })
      .then((response) => {
        if (!active) return
        persistLoginTokens(response)
        navigate(loginEntryPath(response), { replace: true })
      })
      .catch((callbackError) => {
        if (!active) return
        setFormError(userFacingErrorMessage(callbackError, '统一认证登录未能完成,请返回重新发起认证。'))
      })
      .finally(() => {
        if (active) setVerifying(false)
      })
    return () => {
      active = false
    }
  }, [navigate, tenantCode, ticket])

  /** handleCasRedirect 取后端下发的跳转地址后离开本站进入学校统一认证 */
  const handleCasRedirect = useCallback(async () => {
    setRedirecting(true)
    setFormError(null)
    try {
      const { redirect_url } = await api.identity.getCASLoginUrl(tenantCode, ssoServiceUrl(tenantCode))
      const safeRedirectUrl = safeExternalHttpUrl(redirect_url)
      if (!safeRedirectUrl) throw new Error('认证地址无效')
      window.location.assign(safeRedirectUrl)
    } catch (redirectError) {
      setFormError(userFacingErrorMessage(redirectError, '暂时无法前往学校统一认证,请稍后重试。'))
      setRedirecting(false)
    }
  }, [tenantCode])

  /** handleLdapSubmit 用学校目录账号直接登录 */
  const handleLdapSubmit = useCallback(
    async (event: React.FormEvent<HTMLFormElement>) => {
      event.preventDefault()
      const nameOk = setError('username', requiredError(username, '请输入统一认证账号'))
      const passwordOk = setError('password', passwordRequiredError(password, '请输入统一认证密码'))
      if (!nameOk || !passwordOk) return

      setSubmitting(true)
      setFormError(null)
      try {
        const response = await api.identity.ldapLogin(tenantCode, { username: username.trim(), password })
        persistLoginTokens(response)
        navigate(loginEntryPath(response), { replace: true })
      } catch (loginError) {
        setFormError(userFacingErrorMessage(loginError, '统一认证登录失败,请确认账号和密码后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [navigate, password, setError, tenantCode, username],
  )

  if (verifying) {
    return (
      <AuthPanel>
        <div className="flex flex-col items-center gap-4">
          <Icon icon={LoaderCircle} size="lg" className="animate-spin text-accent" />
          <p className="text-sm text-on-dark-sub">正在完成学校统一认证</p>
        </div>
      </AuthPanel>
    )
  }

  return (
    <AuthPanel>
      <div className="w-full max-w-sm">
        <AuthHeading title="学校统一认证" description={`学校代号 ${tenantCode}`} />

        {/* 主通路是去学校的认证系统:这是一次离站导航,不是本页表单的提交 */}
        <Button
          type="button"
          variant="seal"
          size="lg"
          loading={redirecting}
          onClick={handleCasRedirect}
          className="mt-7 w-full"
        >
          前往学校统一认证登录
        </Button>

        {/* 两种方式的分界:文字提示即层级,不做分隔线(FE-6 过渡靠密度) */}
        <p className="mt-8 text-xs uppercase text-on-dark-faint">或使用学校目录账号</p>

        <form onSubmit={handleLdapSubmit} noValidate>
          <AuthTextField
            label="统一认证账号"
            id={`${fieldId}-username`}
            autoComplete="username"
            value={username}
            error={errors.username}
            onValueChange={setUsername}
            onBlur={() => setError('username', requiredError(username, '请输入统一认证账号'))}
          />

          <AuthTextField
            label="统一认证密码"
            id={`${fieldId}-password`}
            type="password"
            autoComplete="current-password"
            value={password}
            error={errors.password}
            onValueChange={setPassword}
            onBlur={() => setError('password', passwordRequiredError(password, '请输入统一认证密码'))}
          />

          <AuthFormError message={formError} />

          {/* 目录账号是次级方式,按钮压到 on-dark 级别,不与上方落印按钮争主次 */}
          <Button type="submit" variant="on-dark" size="lg" loading={submitting} className="mt-7 w-full">
            登录
          </Button>
        </form>

        <AuthQuietLink to="/auth/login" icon={ArrowLeft}>
          返回其他登录方式
        </AuthQuietLink>
      </div>
    </AuthPanel>
  )
}
