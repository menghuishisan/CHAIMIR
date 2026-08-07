// authSession 管理前端登录会话存储和经过角色边界校验的入口恢复。

import type { LoginResponse, TokenRefreshResponse } from '@chaimir/api-client'
import { isRoleHomePath, roleRouteForRoles, type RoleRouteConfig } from './roleRouting'

const MUST_CHANGE_PASSWORD_KEY = 'chaimir.must_change_password'
const PENDING_ENTRY_PATH_KEY = 'chaimir.pending_entry_path'
let accessToken: string | null = null

/**
 * persistLoginTokens 仅在内存保留短期 access token；refresh token 由后端写入 HttpOnly cookie。
 */
export function persistLoginTokens(response: LoginResponse): void {
  accessToken = response.access_token || null
  window.sessionStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
  window.sessionStorage.removeItem(PENDING_ENTRY_PATH_KEY)
  if (response.must_change_pwd) {
    window.sessionStorage.setItem(MUST_CHANGE_PASSWORD_KEY, 'true')
    window.sessionStorage.setItem(PENDING_ENTRY_PATH_KEY, roleEntryPath(response))
  } else {
    window.sessionStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
    window.sessionStorage.removeItem(PENDING_ENTRY_PATH_KEY)
  }
}

/**
 * clearLoginTokens 清除内存 access token 和非敏感会话状态；HttpOnly cookie 由登出接口删除。
 */
export function clearLoginTokens(): void {
  accessToken = null
  window.sessionStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
  window.sessionStorage.removeItem(PENDING_ENTRY_PATH_KEY)
}

/**
 * getStoredAccessToken 读取当前页面内存中的 access token。
 */
export function getStoredAccessToken(): string | null {
  return accessToken
}

/**
 * persistRefreshedTokens 接收轮转后新的短期 access token。
 */
export function persistRefreshedTokens(response: TokenRefreshResponse): void {
  accessToken = response.access_token
}

/**
 * loginEntryPath 把必须改密账号引导到安全拦截页，其余账号进入角色首个功能页。
 */
export function loginEntryPath(response: LoginResponse, requestedPath?: unknown): string {
  if (response.must_change_pwd) return '/auth/change-pwd'
  const roleRoute = accountRoleRoute(response)
  if (!roleRoute) return '/auth/login'
  return safeInternalPath(requestedPath, roleRoute.pathPrefix) || roleRoute.homePath
}

/**
 * accountRoleRoute 取登录响应对应的角色入口。
 * 契约上 account 只在需要选择学校等未完成登录的响应里缺省，此时没有可进入的角色区。
 */
function accountRoleRoute(response: LoginResponse): RoleRouteConfig | undefined {
  if (!response.account) return undefined
  return roleRouteForRoles(response.account.roles)
}

/** safeInternalPath 只接受站内绝对路径，阻止登录回跳被构造成外部地址。 */
function safeInternalPath(value: unknown, rolePrefix: string): string | null {
  if (typeof value !== 'string' || !value.startsWith(`${rolePrefix}/`) || value.startsWith('//') || value.includes('\\')) return null
  return value
}

/**
 * isPasswordChangeRequired 读取登录时保存的服务端改密要求，供路由边界即时拦截。
 */
export function isPasswordChangeRequired(): boolean {
  return window.sessionStorage.getItem(MUST_CHANGE_PASSWORD_KEY) === 'true'
}

/**
 * completeRequiredPasswordChange 清除改密拦截并返回经过白名单校验的角色入口。
 */
export function completeRequiredPasswordChange(): string {
  const pendingPath = window.sessionStorage.getItem(PENDING_ENTRY_PATH_KEY)
  window.sessionStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
  window.sessionStorage.removeItem(PENDING_ENTRY_PATH_KEY)
  return pendingPath && isRoleHomePath(pendingPath) ? pendingPath : '/auth/login'
}

/**
 * roleEntryPath 根据服务端账号角色决定登录后的第一个功能页。
 */
export function roleEntryPath(response: LoginResponse): string {
  const roleRoute = accountRoleRoute(response)
  return roleRoute ? roleRoute.homePath : '/auth/login'
}
