// SSOPage 接入学校 CAS 登录地址和回调，不做本地模拟跳转。

import React, { useCallback, useEffect, useState } from 'react'
import type { ApiError } from '@chaimir/api-client'
import { Button, Input, Spinner } from '@chaimir/ui'
import { ArrowLeft, LogIn } from 'lucide-react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { api } from '../../../../app/api'
import { persistLoginTokens, roleEntryPath } from '../../../../utils/authSession'
import styles from './auth-form.module.css'

/**
 * SSOPage 获取 CAS 登录地址，并处理 CAS 回调完成登录。
 */
const SSOPage: React.FC = () => {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [tenantCode, setTenantCode] = useState(searchParams.get('tenant_code') || '')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const ticket = searchParams.get('ticket')
    const callbackTenantCode = searchParams.get('tenant_code')
    if (!ticket || !callbackTenantCode) {
      return
    }

    let active = true
    setLoading(true)
    setError(null)
    api.identity.casCallback(callbackTenantCode, {
      ticket,
      service: window.location.href.split('?')[0],
    })
      .then((response) => {
        if (!active) {
          return
        }
        persistLoginTokens(response, false)
        navigate(roleEntryPath(response), { replace: true })
      })
      .catch((callbackError) => {
        if (!active) {
          return
        }
        setError((callbackError as ApiError).message || '统一身份认证失败，请重新发起登录')
      })
      .finally(() => {
        if (active) {
          setLoading(false)
        }
      })

    return () => {
      active = false
    }
  }, [navigate, searchParams])

  const handleStartSSO = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const service = `${window.location.origin}/auth/sso?tenant_code=${encodeURIComponent(tenantCode)}`
      const response = await api.identity.getCASLoginUrl(tenantCode, service)
      window.location.assign(response.redirect_url)
    } catch (ssoError) {
      setError((ssoError as ApiError).message || '暂时无法连接学校统一身份认证')
      setLoading(false)
    }
  }, [tenantCode])

  return (
    <div className={styles.form}>
      <div>
        <h2 className={styles.title}>学校统一身份认证</h2>
        <p className={styles.description}>请输入学校代号，系统将跳转到学校认证页面。</p>
      </div>

      {loading && <Spinner label="正在连接统一身份认证" />}
      {error && <div className={styles.error}>{error}</div>}

      <div className={styles.fields}>
        <Input
          fullWidth
          placeholder="学校代号"
          value={tenantCode}
          onChange={(event) => setTenantCode(event.target.value)}
        />
        <Button icon={<LogIn size={16} />} loading={loading} onClick={handleStartSSO}>
          前往统一身份认证
        </Button>
        <Button variant="ghost" icon={<ArrowLeft size={16} />} onClick={() => navigate('/auth/login')}>
          返回登录
        </Button>
      </div>
    </div>
  )
}

export default SSOPage
