// TenantSelectPage 多学校选择页:手机号绑定多校时,选校后携 tenant_id 重新完成登录。
// 凭证来自内存暂存(pendingLogin),刷新即失效——此时如实引导重新登录,凭证绝不持久化。

import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router'
import { ArrowLeft, Building2 } from 'lucide-react'
import { Button, Icon } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { loginEntryPath, persistLoginTokens } from '../../../../utils/authSession'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { AuthFormError, AuthHeading, AuthPanel, AuthQuietAction } from '../../components/AuthUI'
import { clearPendingTenantLogin, getPendingTenantLogin } from './pendingLogin'

/**
 * TenantSelectPage 列出可选学校;选择即重新登录进入对应租户。
 */
export default function TenantSelectPage() {
  const navigate = useNavigate()
  // 组件生命周期内凭证不变,读一次即可(状态初始化)
  const [pending] = useState(getPendingTenantLogin)
  const [submittingTenantId, setSubmittingTenantId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  /** backToLogin 清除暂存凭证并返回登录页 */
  const backToLogin = useCallback(() => {
    clearPendingTenantLogin()
    navigate('/auth/login', { replace: true })
  }, [navigate])

  /** handleSelect 携所选租户重新提交登录 */
  const handleSelect = useCallback(
    async (tenantId: string) => {
      if (!pending) return
      setSubmittingTenantId(tenantId)
      setError(null)
      try {
        const response =
          pending.method.type === 'phone'
            ? await api.identity.loginPhone({
                phone: pending.method.phone,
                password: pending.method.password,
                tenant_id: tenantId,
                remember: pending.remember,
              })
            : await api.identity.loginSMS({
                phone: pending.method.phone,
                code: pending.method.code,
                tenant_id: tenantId,
                remember: pending.remember,
              })
        clearPendingTenantLogin()
        persistLoginTokens(response)
        navigate(loginEntryPath(response, pending.returnPath), { replace: true })
      } catch (selectError) {
        setError(userFacingErrorMessage(selectError, '无法进入所选学校,请稍后重试。'))
      } finally {
        setSubmittingTenantId(null)
      }
    },
    [navigate, pending],
  )

  // 直接访问本页或刷新后凭证已失效:如实引导重新登录
  if (!pending || pending.tenants.length === 0) {
    return (
      <AuthPanel>
        <div className="w-full max-w-sm">
          <AuthHeading
            title="需要重新登录"
            description="登录状态已失效(页面刷新会清除待选学校信息),请返回登录页重新认证。"
          />
          <Button variant="on-dark" size="md" leftIcon={ArrowLeft} className="mt-7" onClick={backToLogin}>
            返回登录
          </Button>
        </div>
      </AuthPanel>
    )
  }

  return (
    <AuthPanel>
      <div className="w-full max-w-sm">
        <AuthHeading
          title="选择你的学校"
          description="这个手机号绑定了多个学校,请选择本次要进入的学校。"
        />

        <AuthFormError message={error} />

        <ul className="mt-6 flex flex-col gap-2.5">
          {pending.tenants.map((tenant) => {
            const submitting = submittingTenantId === tenant.tenant_id
            return (
              <li key={tenant.tenant_id}>
                <button
                  type="button"
                  disabled={submittingTenantId !== null}
                  onClick={() => handleSelect(tenant.tenant_id)}
                  className="pressable flex w-full items-center gap-3 rounded-lg border border-dark-line bg-dark-surface px-4 py-3 text-left hover:border-on-dark-accent-line hover:bg-dark-elevated disabled:pointer-events-none disabled:opacity-50"
                >
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-on-dark-accent-soft">
                    <Icon icon={Building2} size="sm" className="text-accent" />
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-base font-medium text-on-dark">{tenant.name}</span>
                    <span className="block font-mono text-xs text-on-dark-sub">{tenant.code}</span>
                  </span>
                  {submitting ? <span className="text-xs text-on-dark-sub">正在进入…</span> : null}
                </button>
              </li>
            )
          })}
        </ul>

        <AuthQuietAction icon={ArrowLeft} onClick={backToLogin}>
          返回登录
        </AuthQuietAction>
      </div>
    </AuthPanel>
  )
}
