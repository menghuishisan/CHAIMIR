// @chaimir/auth 主入口：集中导出认证入口、认证上下文、路由守卫和角色权限工具。

export { AuthApp } from './AuthApp'
export { AuthGate } from './AuthGate'
export type { AuthGateProps } from './AuthGate'

export { AuthProvider, useAuth } from './AuthContext'
export type { AuthContextValue, AuthProviderProps } from './AuthContext'

export { ProtectedRoute } from './ProtectedRoute'
export type { ProtectedRouteProps } from './ProtectedRoute'

export * from './permissions'
