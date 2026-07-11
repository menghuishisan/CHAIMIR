// SSOPage 提供学校 CAS 网页认证与 LDAP 学校账号认证，并处理 CAS 回调。

import React, { useCallback, useEffect, useState } from 'react'
import { Button, Checkbox, FormField, Input, Spinner, Tabs } from '@chaimir/ui'
import { ArrowLeft, ExternalLink, LogIn } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../../../../app/api'
import { loginEntryPath, persistLoginTokens } from '../../../../utils/authSession'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import styles from './auth-form.module.css'
import publicStyles from './public-auth.module.css'

type SSOLoginMethod = 'cas' | 'ldap'

const SSO_METHODS = [
  { key: 'cas', label: '网页统一认证' },
  { key: 'ldap', label: '学校账号认证' },
]

/**
 * SSOPage 获取 CAS 登录地址、处理 CAS 回调，并支持 LDAP 账号密码登录。
 */
const SSOPage: React.FC = () => {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [method, setMethod] = useState<SSOLoginMethod>('cas')
  const [tenantCode, setTenantCode] = useState(searchParams.get('tenant_code') || '')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [remember, setRemember] = useState(searchParams.get('remember') === '1')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const returnPath = searchParams.get('return_to') || undefined

  useEffect(() => {
    const ticket = searchParams.get('ticket')
    const callbackTenantCode = searchParams.get('tenant_code')
    if (!ticket || !callbackTenantCode) {
      return
    }

    let active = true
    const serviceURL = new URL(window.location.href)
    serviceURL.searchParams.delete('ticket')
    setLoading(true)
    setError(null)
    api.identity.casCallback(callbackTenantCode, { ticket, service: serviceURL.toString() })
      .then((response) => {
        if (!active) {
          return
        }
        persistLoginTokens(response, searchParams.get('remember') === '1')
        navigate(loginEntryPath(response, returnPath), { replace: true })
      })
      .catch((callbackError) => {
        if (active) {
          setError(userFacingErrorMessage(callbackError, '学校统一认证失败，请重新发起登录。'))
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false)
        }
      })

    return () => {
      active = false
    }
  }, [navigate, returnPath, searchParams])

  /**
   * handleStartCAS 从后端获取学校认证地址，再交给浏览器完成跳转。
   */
  const handleStartCAS = useCallback(async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const normalizedTenantCode = tenantCode.trim()
    if (!normalizedTenantCode) {
      setError('请输入学校代号。')
      return
    }
    setLoading(true)
    setError(null)
    try {
      const serviceURL = new URL('/auth/sso', window.location.origin)
      serviceURL.searchParams.set('tenant_code', normalizedTenantCode)
      if (remember) {
        serviceURL.searchParams.set('remember', '1')
      }
      if (returnPath) serviceURL.searchParams.set('return_to', returnPath)
      const response = await api.identity.getCASLoginUrl(normalizedTenantCode, serviceURL.toString())
      window.location.assign(response.redirect_url)
    } catch (ssoError) {
      setError(userFacingErrorMessage(ssoError, '暂时无法连接学校统一认证，请稍后重试。'))
      setLoading(false)
    }
  }, [remember, returnPath, tenantCode])

  /**
   * handleLDAPLogin 把学校账号凭证提交给后端 LDAP 认证入口。
   */
  const handleLDAPLogin = useCallback(async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const normalizedTenantCode = tenantCode.trim()
    if (!normalizedTenantCode || !username.trim() || !password) {
      setError('请输入学校代号、统一认证账号和密码。')
      return
    }
    setLoading(true)
    setError(null)
    try {
      const response = await api.identity.ldapLogin(normalizedTenantCode, {
        username: username.trim(),
        password,
      })
      persistLoginTokens(response, remember)
      navigate(loginEntryPath(response, returnPath), { replace: true })
    } catch (ldapError) {
      setError(userFacingErrorMessage(ldapError, '学校账号认证失败，请检查账号信息后重试。'))
    } finally {
      setLoading(false)
    }
  }, [navigate, password, remember, returnPath, tenantCode, username])

  return (
    <main className={publicStyles.publicPage}>
      <section className={`${publicStyles.publicCard} ${publicStyles.compactCard}`} aria-labelledby="sso-title">
        <div className={styles.form}>
          <div>
            <h1 id="sso-title" className={styles.title}>学校统一身份认证</h1>
            <p className={styles.description}>选择学校支持的认证方式登录。</p>
          </div>

          {loading && <Spinner label="正在连接学校认证服务" />}
          {error && <div className={styles.error} role="alert">{error}</div>}

          <Tabs className={styles.ssoTabs} items={SSO_METHODS} activeKey={method} ariaLabel="学校认证方式" onChange={(key) => {
            setMethod(key as SSOLoginMethod)
            setError(null)
          }}>
            {method === 'cas' ? (
              <form className={styles.fields} onSubmit={handleStartCAS}>
                <FormField label="学校代号" htmlFor="cas-tenant-code" helperText="由学校管理员提供" required>
                  <Input id="cas-tenant-code" fullWidth autoComplete="organization" value={tenantCode} onChange={(event) => setTenantCode(event.target.value)} />
                </FormField>
                <Button type="submit" block icon={<ExternalLink size={16} />} loading={loading}>前往学校认证页面</Button>
              </form>
            ) : (
              <form className={styles.fields} onSubmit={handleLDAPLogin}>
                <FormField label="学校代号" htmlFor="ldap-tenant-code" helperText="由学校管理员提供" required>
                  <Input id="ldap-tenant-code" fullWidth autoComplete="organization" value={tenantCode} onChange={(event) => setTenantCode(event.target.value)} />
                </FormField>
                <FormField label="统一认证账号" htmlFor="ldap-username" required>
                  <Input id="ldap-username" fullWidth autoComplete="username" value={username} onChange={(event) => setUsername(event.target.value)} />
                </FormField>
                <FormField label="统一认证密码" htmlFor="ldap-password" required>
                  <Input id="ldap-password" fullWidth type="password" autoComplete="current-password" value={password} onChange={(event) => setPassword(event.target.value)} />
                </FormField>
                <Button type="submit" block icon={<LogIn size={16} />} loading={loading}>登录</Button>
              </form>
            )}
          </Tabs>

          <div className={styles.options}>
            <Checkbox checked={remember} label="保持登录" onChange={(event) => setRemember(event.target.checked)} />
          </div>
          <Button variant="ghost" icon={<ArrowLeft size={16} />} onClick={() => navigate('/auth/login', { state: { from: returnPath } })}>返回登录</Button>
        </div>
      </section>
    </main>
  )
}

export default SSOPage
