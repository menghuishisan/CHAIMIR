// RoleGuard 在路由边界校验服务端会话角色，阻止越权进入其他端页面。

import React from 'react'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import type { UserRole } from '@chaimir/api-client'
import { api } from '../../app/api'
import { ErrorState, LoadingState } from '../ResourceState'
import { useAsyncResource } from '../../hooks'
import { isPasswordChangeRequired } from '../../utils/authSession'

export interface RoleGuardProps {
  allowedRoles: UserRole[]
}

/**
 * RoleGuard 每次进入受保护路由时读取当前账号角色并执行服务端会话校验。
 */
export const RoleGuard: React.FC<RoleGuardProps> = ({ allowedRoles }) => {
  if (isPasswordChangeRequired()) {
    return <Navigate to="/auth/change-pwd" replace />
  }

  return <VerifiedRoleGuard allowedRoles={allowedRoles} />
}

/**
 * VerifiedRoleGuard 向服务端读取当前账号，校验角色与会话仍然有效。
 */
const VerifiedRoleGuard: React.FC<RoleGuardProps> = ({ allowedRoles }) => {
  const location = useLocation()
  const resource = useAsyncResource(() => api.identity.getMe(), [])
  const roles = resource.data?.account.roles || []
  const allowed = roles.some((role) => allowedRoles.includes(role))

  if (resource.status === 'loading') {
    return <LoadingState title="正在校验访问权限" />
  }

  if (resource.status === 'error') {
    if (resource.error?.status === 401) {
      return <Navigate to="/auth/login" replace state={{ from: location.pathname }} />
    }
    return <ErrorState error={resource.error} onRetry={resource.reload} title="暂时无法校验访问权限" />
  }

  if (!allowed) {
    return <Navigate to="/auth/login" replace state={{ from: location.pathname }} />
  }

  return <Outlet />
}
