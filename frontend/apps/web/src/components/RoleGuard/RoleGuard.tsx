// RoleGuard 在路由边界校验服务端会话与角色:
// 未登录 → 登录页(携带回跳路径);角色不匹配 → 403(保留 URL,不再误踢登录页,根治审查 S6);
// 校验通过 → 经 SessionContext 下发账号供壳层与页面复用。

import React from 'react'
import { Navigate, Outlet } from 'react-router'
import { LoaderCircle, ShieldAlert } from 'lucide-react'
import type { UserRole } from '@chaimir/api-client'
import { Button } from '@chaimir/ui'
import { api } from '../../app/api'
import { useAsyncResource } from '../../hooks'
import { isPasswordChangeRequired } from '../../utils/authSession'
import { AppStatusScreen } from '../AppStatusScreen'
import { ForbiddenPage } from '../StatusPages'
import { SessionContext } from './session'

interface RoleGuardProps {
  allowedRoles: UserRole[]
}

/**
 * RoleGuard 只执行本地改密拦截，认证状态统一由 /me 与 HttpOnly refresh cookie 恢复。
 */
export const RoleGuard: React.FC<RoleGuardProps> = ({ allowedRoles }) => {
  // 服务端要求改密的账号先完成改密,再进入任何受保护页面
  if (isPasswordChangeRequired()) {
    return <Navigate to="/auth/change-pwd" replace />
  }
  return <VerifiedRoleGuard allowedRoles={allowedRoles} />
}

/**
 * VerifiedRoleGuard 读取 /me 校验会话有效性与角色归属。
 */
const VerifiedRoleGuard: React.FC<RoleGuardProps> = ({ allowedRoles }) => {
  const resource = useAsyncResource(() => api.identity.getMe(), [])

  if (resource.status === 'loading') {
    return <AppStatusScreen icon={LoaderCircle} spinning title="正在校验访问权限" />
  }

  if (resource.status === 'error' || !resource.data) {
    // 会话失效不在此处判定:后端签发登录失效码时,API 客户端的 onUnauthorized
    // 已清令牌并按所在角色区跳登录(app/api.ts),全站单一出口。
    // 这里只表达「校验没能完成」这一类可重试失败,并带出后端报障编号。
    return (
      <AppStatusScreen
        icon={ShieldAlert}
        tone="danger"
        title="暂时无法校验访问权限"
        description="请检查网络后重试;若持续失败,请联系本校管理员。"
        traceId={resource.error?.traceId}
        actions={
          <Button variant="on-dark" onClick={resource.reload}>
            重新校验
          </Button>
        }
      />
    )
  }

  const roles = resource.data.account.roles
  const allowed = roles.some((role) => allowedRoles.includes(role))
  if (!allowed) {
    // 已登录但角色不符:保留当前 URL 展示 403,提供回到本人首页的出口
    return <ForbiddenPage roles={roles} />
  }

  return (
    <SessionContext.Provider value={{ me: resource.data, reload: resource.reload }}>
      <Outlet />
    </SessionContext.Provider>
  )
}
