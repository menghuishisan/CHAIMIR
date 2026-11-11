// TenantSelectPage 多学校选择页:手机号绑定多校时,选校后携 tenant_id 重新完成登录。
// 凭证来自内存暂存(pendingLogin),刷新即失效——此时如实引导重新登录,凭证绝不持久化。

import { useCallback, useState } from 'react'
import { useNavigate } from 'react-router'
import { ArrowLeft, ChevronRight, LoaderCircle } from 'lucide-react'
import { Button, Icon, TenantCrest } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { loginEntryPath, persistLoginTokens } from '../../../../utils/authSession'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { AuthFormBrand, AuthFormError, AuthHeading, AuthPanel, AuthQuietAction } from '../../components/AuthUI'
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
        <AuthFormBrand />
        <div className="w-full max-w-sm">
          <AuthHeading
            title="登录状态已失效,请重新登录"
            description="为了安全,待选学校的信息不会保留到刷新之后。回到登录页重新输入即可。"
          />
          <Button variant="primary" size="lg" leftIcon={ArrowLeft} className="mt-7 w-full" onClick={backToLogin}>
            返回登录
          </Button>
        </div>
      </AuthPanel>
    )
  }

  return (
    <AuthPanel>
      <AuthFormBrand />
      <div className="w-full max-w-sm">
        <AuthHeading
          title="选择要进入的学校"
          description="这个手机号在几所学校都有账号,选一所继续登录。"
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
                  aria-busy={submitting || undefined}
                  className="pressable flex w-full items-center gap-3 rounded-lg bg-dark-elevated px-4 py-3.5 text-left hover:bg-dark-surface focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:pointer-events-none disabled:opacity-60"
                >
                  <TenantCrest name={tenant.name} size="sm" onDark />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-base font-medium text-on-dark">{tenant.name}</span>
                    <span className="block font-mono text-xs text-on-dark-sub">{tenant.code}</span>
                  </span>
                  {submitting ? (
                    <Icon icon={LoaderCircle} size="sm" className="shrink-0 animate-spin text-accent" label="正在进入" />
                  ) : (
                    <Icon icon={ChevronRight} size="sm" className="shrink-0 text-on-dark-sub" />
                  )}
                </button>
              </li>
            )
          })}
        </ul>

        <AuthQuietAction icon={ArrowLeft} onClick={backToLogin}>
          换个账号登录
        </AuthQuietAction>
      </div>
    </AuthPanel>
  )
}
