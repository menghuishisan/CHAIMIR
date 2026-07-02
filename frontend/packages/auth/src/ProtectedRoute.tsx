// 路由守卫组件：按登录态、角色和自定义权限函数控制前端路由可见性。

import React from 'react'
import { useAuth } from './AuthContext'
import type { Account } from '@chaimir/api-client'
import { UserRole } from '@chaimir/shared'

export interface ProtectedRouteProps {
  /** 子元素 */
  children: React.ReactNode
  /** 需要的角色（任一匹配即可） */
  requiredRoles?: UserRole[]
  /** 未授权时的回退组件 */
  fallback?: React.ReactNode
  /** 自定义权限检查函数 */
  checkPermission?: (user: Account) => boolean
}

/**
 * ProtectedRoute 根据登录态、角色列表和可选业务权限函数保护页面内容。
 */
export const ProtectedRoute: React.FC<ProtectedRouteProps> = ({
  children,
  requiredRoles,
  fallback,
  checkPermission,
}) => {
  const { user, isAuthenticated, isLoading } = useAuth()

  // 认证状态加载期间给读屏和键盘用户明确反馈。
  if (isLoading) {
    return <RouteStateMessage role="status" message="正在加载，请稍候" />
  }

  // 未登录时不暴露页面内容,由调用方决定是否传入跳转组件。
  if (!isAuthenticated) {
    return fallback || <RouteStateMessage role="alert" message="请先登录后继续" />
  }

  // 到这里 user 必须存在,否则视为认证状态不完整并拦截访问。
  if (!user) {
    return fallback || <RouteStateMessage role="alert" message="请先登录后继续" />
  }

  // 自定义权限检查用于页面级业务规则,例如课程归属或竞赛报名状态。
  if (checkPermission && !checkPermission(user)) {
    return fallback || <RouteStateMessage role="alert" message="你没有访问该页面的权限" />
  }

  // 角色检查只接受服务端会话返回的角色,不信任页面参数。
  if (requiredRoles && requiredRoles.length > 0) {
    const hasPermission = requiredRoles.some((role) => user.roles.includes(role))
    if (!hasPermission) {
      return fallback || <RouteStateMessage role="alert" message="你没有访问该页面的权限" />
    }
  }

  return <>{children}</>
}

/**
 * RouteStateMessage 渲染路由守卫默认状态文案,避免把空白页留给用户。
 */
function RouteStateMessage({ role, message }: { role: 'status' | 'alert'; message: string }) {
  return <div role={role}>{message}</div>
}
