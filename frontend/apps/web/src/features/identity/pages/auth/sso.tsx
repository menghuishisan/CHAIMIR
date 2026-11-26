// SsoCallbackPage 统一认证中转页(公共页,路径 /auth/sso/:tenantCode):
// 一页承担两件事 ——
//   1) 从学校统一认证跳回时(URL 带 ticket):验票并由会话能力恢复目标或进入角色首页;
//   2) 直接进入本页时:提供 CAS 跳转入口与学校目录账号(LDAP)登录两种方式。
// service 参数固定为本页自身地址(去掉 ticket 查询串),后端按来源白名单校验,
// 前端不拼接任何外部地址,避免开放重定向。

import React, { useCallback, useEffect, useId, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { Button, Icon } from '@chaimir/ui'
import { ArrowLeft, LoaderCircle } from 'lucide-react'
import { api } from '../../../../app/api'
import { completeLogin } from '../../../../utils/authSession'
import { safeExternalHttpsUrl } from '../../../../utils/safeNavigation'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { passwordRequiredError, requiredError } from '../../authForm'
import { useFormValidation } from '../../../../hooks/useFormValidation'
import {
  AuthFormError,
  AuthHeading,
  AuthPanel,
  AuthQuietLink,
  AuthTextField,
} from '../../components/AuthUI'

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

  const [settledVerification, setSettledVerification] = useState<{ tenantCode: string; ticket: string } | null>(null)
  const verifying = Boolean(ticket && tenantCode &&
    (settledVerification?.tenantCode !== tenantCode || settledVerification.ticket !== ticket))
  const [redirecting, setRedirecting] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const { errors, validate } = useFormValidation(() => ({
    username: requiredError(username, '请输入统一认证账号'),
    password: passwordRequiredError(password, '请输入统一认证密码'),
  }))
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  // 票据只能消费一次；保留请求供 Effect 重新订阅，学校与票据共同标识一次验票。
  const verification = useRef<{
    tenantCode: string
    ticket: string
    request: ReturnType<typeof api.identity.casCallback>
  } | null>(null)

  // 带票据进入即自动验票换会话
  useEffect(() => {
    if (!ticket || !tenantCode) return
    if (verification.current?.tenantCode !== tenantCode || verification.current.ticket !== ticket) {
      verification.current = {
        tenantCode,
        ticket,
        request: api.identity.casCallback(tenantCode, { ticket, service: ssoServiceUrl(tenantCode) }),
      }
    }
    let active = true
    verification.current.request
      .then((response) => {
        if (!active) return
        navigate(completeLogin(response), { replace: true })
      })
      .catch((callbackError) => {
        if (!active) return
        setFormError(userFacingErrorMessage(callbackError, '统一认证登录未能完成,请返回重新发起认证。'))
      })
      .finally(() => {
        if (active) setSettledVerification({ tenantCode, ticket })
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
      const safeRedirectUrl = safeExternalHttpsUrl(redirect_url)
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
      if (!validate()) return

      setSubmitting(true)
      setFormError(null)
      try {
        const response = await api.identity.ldapLogin(tenantCode, { username: username.trim(), password })
        navigate(completeLogin(response), { replace: true })
      } catch (loginError) {
        setFormError(userFacingErrorMessage(loginError, '统一认证登录失败,请确认账号和密码后重试。'))
      } finally {
        setSubmitting(false)
      }
    },
    [navigate, password, tenantCode, username, validate],
  )

  if (verifying) {
    return (
      <AuthPanel>
        <div className="flex flex-col items-center gap-4">
          <Icon icon={LoaderCircle} size="lg" className="animate-spin text-accent" />
          <p className="text-base text-on-dark">正在完成学校统一认证</p>
          <p className="text-sm text-on-dark-sub">认证通过后会自动进入平台,请稍候。</p>
        </div>
      </AuthPanel>
    )
  }

  return (
    <AuthPanel>
      <div className="w-full max-w-sm">
        <AuthHeading title="学校统一认证" description={`学校代号 ${tenantCode} · 用本校的统一身份认证账号登录`} />

        <Button
          type="button"
          variant="primary"
          size="lg"
          loading={redirecting}
          onClick={handleCasRedirect}
          className="mt-7 w-full"
        >
          前往学校统一认证登录
        </Button>

        <div className="mt-8 flex items-center gap-3 text-sm text-on-dark-sub">
          <span aria-hidden="true" className="h-px flex-1 bg-dark-line" />
          <span>或用学校目录账号登录</span>
          <span aria-hidden="true" className="h-px flex-1 bg-dark-line" />
        </div>

        <form onSubmit={handleLdapSubmit} noValidate>
          <AuthTextField
            label="统一认证账号"
            id={`${fieldId}-username`}
            autoComplete="username"
            value={username}
            error={errors.username}
            onValueChange={setUsername}
            onBlur={() => validate(['username'])}
          />

          <AuthTextField
            label="统一认证密码"
            id={`${fieldId}-password`}
            type="password"
            autoComplete="current-password"
            value={password}
            error={errors.password}
            onValueChange={setPassword}
            onBlur={() => validate(['password'])}
          />

          <AuthFormError message={formError} />

          <Button type="submit" variant="on-dark" size="lg" loading={submitting} className="mt-7 w-full">
            登录
          </Button>
        </form>

        <AuthQuietLink to="/auth/login" icon={ArrowLeft}>
          用手机号登录
        </AuthQuietLink>
      </div>
    </AuthPanel>
  )
}
