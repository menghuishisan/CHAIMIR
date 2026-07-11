// TenantSelectPage 处理手机号登录返回的多租户选择，不展示静态学校数据。

import React, { useCallback, useMemo, useState } from 'react'
import { Button } from '@chaimir/ui'
import { ArrowLeft, Building } from 'lucide-react'
import { useLocation, useNavigate } from 'react-router-dom'
import { api } from '../../../../app/api'
import { loginEntryPath, persistLoginTokens } from '../../../../utils/authSession'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import type { TenantSelectLoginState } from './login'
import styles from './auth-form.module.css'

/**
 * TenantSelectPage 选择租户后重新完成手机号登录。
 */
const TenantSelectPage: React.FC = () => {
  const navigate = useNavigate()
  const location = useLocation()
  const state = location.state as TenantSelectLoginState | null
  const tenants = useMemo(() => state?.tenants || [], [state])
  const [submittingTenantId, setSubmittingTenantId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const handleSelectTenant = useCallback(async (tenantId: string) => {
    if (!state) {
      navigate('/auth/login', { replace: true })
      return
    }
    setSubmittingTenantId(tenantId)
    setError(null)
    try {
      const numericTenantId = Number(tenantId)
      const response = state.method.type === 'phone'
        ? await api.identity.loginPhone({
            phone: state.method.phone,
            password: state.method.password,
            tenant_id: numericTenantId,
          })
        : await api.identity.loginSMS({
            phone: state.method.phone,
            code: state.method.code,
            tenant_id: numericTenantId,
          })
      persistLoginTokens(response, state.remember)
      navigate(loginEntryPath(response, state.returnPath), { replace: true })
    } catch (selectError) {
      setError(userFacingErrorMessage(selectError, '无法进入所选学校，请稍后重试。'))
    } finally {
      setSubmittingTenantId(null)
    }
  }, [navigate, state])

  if (!state || tenants.length === 0) {
    return (
      <div className={styles.form}>
        <div>
          <h2 className={styles.title}>需要重新登录</h2>
          <p className={styles.description}>当前没有可选择的学校，请返回登录页重新认证。</p>
        </div>
        <Button variant="outline" icon={<ArrowLeft size={16} />} onClick={() => navigate('/auth/login', { replace: true })}>
          返回登录
        </Button>
      </div>
    )
  }

  return (
    <div className={styles.form}>
      <div>
        <h2 className={styles.title}>选择您的学校</h2>
        <p className={styles.description}>您的手机号绑定了多个学校，请选择本次要进入的学校系统。</p>
      </div>

      {error && <div className={styles.error} role="alert">{error}</div>}

      <div className={styles.tenantList}>
        {tenants.map((tenant) => (
          <button
            className={styles.tenantButton}
            key={tenant.tenant_id}
            type="button"
            onClick={() => handleSelectTenant(tenant.tenant_id)}
          >
            <span className={styles.tenantIcon} aria-hidden="true">
              <Building size={20} />
            </span>
            <span>
              <span className={styles.tenantName}>{tenant.name}</span>
              <span className={styles.tenantCode}>{tenant.code}</span>
            </span>
            {submittingTenantId === tenant.tenant_id && <span className={styles.tenantCode}>正在进入</span>}
          </button>
        ))}
      </div>

      <Button variant="ghost" icon={<ArrowLeft size={16} />} onClick={() => navigate('/auth/login', { replace: true })}>
        返回重新登录
      </Button>
    </div>
  )
}

export default TenantSelectPage
