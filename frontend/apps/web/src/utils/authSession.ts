// authSession 管理前端登录会话落点，统一 token 存储键名和角色入口跳转。

import { UserRole } from '@chaimir/api-client'
import type { LoginResponse } from '@chaimir/api-client'

export const ACCESS_TOKEN_KEY = 'chaimir.access_token'
export const REFRESH_TOKEN_KEY = 'chaimir.refresh_token'
const MUST_CHANGE_PASSWORD_KEY = 'chaimir.must_change_password'
const PENDING_ENTRY_PATH_KEY = 'chaimir.pending_entry_path'

const ROLE_ENTRY_PATHS = new Set([
  '/platform-admin/schools',
  '/school-admin/users',
  '/teacher/courses',
  '/student/courses',
])

/**
 * persistLoginTokens 按用户选择把登录令牌写入同一种键名。
 */
export function persistLoginTokens(response: LoginResponse, remember: boolean): void {
  const storage = remember ? window.localStorage : window.sessionStorage
  const otherStorage = remember ? window.sessionStorage : window.localStorage
  otherStorage.removeItem(ACCESS_TOKEN_KEY)
  otherStorage.removeItem(REFRESH_TOKEN_KEY)
  otherStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
  otherStorage.removeItem(PENDING_ENTRY_PATH_KEY)
  if (response.access_token) {
    storage.setItem(ACCESS_TOKEN_KEY, response.access_token)
  }
  if (response.refresh_token) {
    storage.setItem(REFRESH_TOKEN_KEY, response.refresh_token)
  }
  if (response.must_change_pwd) {
    storage.setItem(MUST_CHANGE_PASSWORD_KEY, 'true')
    storage.setItem(PENDING_ENTRY_PATH_KEY, roleEntryPath(response))
  } else {
    storage.removeItem(MUST_CHANGE_PASSWORD_KEY)
    storage.removeItem(PENDING_ENTRY_PATH_KEY)
  }
}

/**
 * clearLoginTokens 清除浏览器两类存储中的登录令牌。
 */
export function clearLoginTokens(): void {
  window.localStorage.removeItem(ACCESS_TOKEN_KEY)
  window.localStorage.removeItem(REFRESH_TOKEN_KEY)
  window.sessionStorage.removeItem(ACCESS_TOKEN_KEY)
  window.sessionStorage.removeItem(REFRESH_TOKEN_KEY)
  window.localStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
  window.localStorage.removeItem(PENDING_ENTRY_PATH_KEY)
  window.sessionStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
  window.sessionStorage.removeItem(PENDING_ENTRY_PATH_KEY)
}

/**
 * loginEntryPath 把必须改密账号引导到安全拦截页，其余账号进入角色首个功能页。
 */
export function loginEntryPath(response: LoginResponse): string {
  return response.must_change_pwd ? '/auth/change-pwd' : roleEntryPath(response)
}

/**
 * isPasswordChangeRequired 读取登录时保存的服务端改密要求，供路由边界即时拦截。
 */
export function isPasswordChangeRequired(): boolean {
  return window.localStorage.getItem(MUST_CHANGE_PASSWORD_KEY) === 'true'
    || window.sessionStorage.getItem(MUST_CHANGE_PASSWORD_KEY) === 'true'
}

/**
 * completeRequiredPasswordChange 清除改密拦截并返回经过白名单校验的角色入口。
 */
export function completeRequiredPasswordChange(): string {
  const pendingPath = window.localStorage.getItem(PENDING_ENTRY_PATH_KEY)
    || window.sessionStorage.getItem(PENDING_ENTRY_PATH_KEY)
  window.localStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
  window.localStorage.removeItem(PENDING_ENTRY_PATH_KEY)
  window.sessionStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
  window.sessionStorage.removeItem(PENDING_ENTRY_PATH_KEY)
  return pendingPath && ROLE_ENTRY_PATHS.has(pendingPath) ? pendingPath : '/auth/login'
}

/**
 * roleEntryPath 根据服务端账号角色决定登录后的第一个功能页。
 */
export function roleEntryPath(response: LoginResponse): string {
  const roles = response.account?.roles || []
  if (roles.includes(UserRole.PLATFORM_ADMIN)) {
    return '/platform-admin/schools'
  }
  if (roles.includes(UserRole.SCHOOL_ADMIN)) {
    return '/school-admin/users'
  }
  if (roles.includes(UserRole.TEACHER)) {
    return '/teacher/courses'
  }
  return '/student/courses'
}
