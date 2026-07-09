// authSession 管理前端登录会话落点，统一 token 存储键名和角色入口跳转。

import { UserRole } from '@chaimir/api-client'
import type { LoginResponse } from '@chaimir/api-client'

export const ACCESS_TOKEN_KEY = 'chaimir.access_token'
export const REFRESH_TOKEN_KEY = 'chaimir.refresh_token'

/**
 * persistLoginTokens 按用户选择把登录令牌写入同一种键名。
 */
export function persistLoginTokens(response: LoginResponse, remember: boolean): void {
  const storage = remember ? window.localStorage : window.sessionStorage
  const otherStorage = remember ? window.sessionStorage : window.localStorage
  otherStorage.removeItem(ACCESS_TOKEN_KEY)
  otherStorage.removeItem(REFRESH_TOKEN_KEY)
  if (response.access_token) {
    storage.setItem(ACCESS_TOKEN_KEY, response.access_token)
  }
  if (response.refresh_token) {
    storage.setItem(REFRESH_TOKEN_KEY, response.refresh_token)
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
