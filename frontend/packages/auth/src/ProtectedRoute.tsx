// 路由守卫组件

import React from 'react'
import { useAuth } from './AuthContext'
import { UserRole } from '@chaimir/shared'

export interface ProtectedRouteProps {
  /** 子元素 */
  children: React.ReactNode
  /** 需要的角色（任一匹配即可） */
  requiredRoles?: UserRole[]
  /** 未授权时的回退组件 */
  fallback?: React.ReactNode
  /** 自定义权限检查函数 */
  checkPermission?: (user: any) => boolean
}

/**
 * ProtectedRoute：路由守卫
 * 只有满足权限要求的用户才能访问
 */
export const ProtectedRoute: React.FC<ProtectedRouteProps> = ({
  children,
  requiredRoles,
  fallback,
  checkPermission,
}) => {
  const { user, isAuthenticated, isLoading } = useAuth()

  // 加载中
  if (isLoading) {
    return <div>加载中...</div>
  }

  // 未登录
  if (!isAuthenticated) {
    return fallback || <div>请先登录</div>
  }

  // 自定义权限检查
  if (checkPermission && !checkPermission(user)) {
    return fallback || <div>无权访问</div>
  }

  // 角色检查
  if (requiredRoles && requiredRoles.length > 0) {
    const hasPermission = requiredRoles.some((role) => user?.roles.includes(role))
    if (!hasPermission) {
      return fallback || <div>无权访问</div>
    }
  }

  return <>{children}</>
}
