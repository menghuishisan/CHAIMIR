// authSession 管理前端登录会话存储和经过角色边界校验的入口恢复。

import type { LoginResponse, TokenRefreshResponse, UserRole } from '@chaimir/api-client'
import { ROLE_ROUTES, roleRouteForRoles } from './roleRouting'
import { safeInternalNavigation } from './safeNavigation'
import { errorDiagnostics } from './userFacingError'

const MUST_CHANGE_PASSWORD_KEY = 'chaimir.must_change_password'
const PENDING_ENTRY_PATH_KEY = 'chaimir.pending_entry_path'
const AUTH_SESSION_CHANNEL = 'chaimir-auth-session'
let accessToken: string | null = null
let mustChangePassword = false
let pendingEntryPath: string | null = null
let authChannel: BroadcastChannel | undefined
const authSessionListeners = new Set<() => void>()

/** authSessionChannel 初始化只传递固定变更信号,不广播账号、租户或令牌。 */
function authSessionChannel(): BroadcastChannel | undefined {
  if (authChannel) return authChannel
  if (typeof BroadcastChannel === 'undefined') return undefined
  try {
    authChannel = new BroadcastChannel(AUTH_SESSION_CHANNEL)
    authChannel.addEventListener('message', (event: MessageEvent<unknown>) => {
      if (!isAuthSessionChangedMessage(event.data)) return
      for (const listener of authSessionListeners) listener()
    })
    return authChannel
  } catch (error) {
    console.warn('auth_session_broadcast_unavailable', {
      operation: 'initialize_auth_session_channel',
      error: errorDiagnostics(error),
    })
    return undefined
  }
}

/** isAuthSessionChangedMessage 只接受固定的无敏感字段消息。 */
function isAuthSessionChangedMessage(value: unknown): value is { type: 'session-changed' } {
  return (
    typeof value === 'object' &&
    value !== null &&
    'type' in value &&
    value.type === 'session-changed'
  )
}

/** notifyAuthSessionChanged 通知其他标签重新认证,不传递任何凭据或身份信息。 */
function notifyAuthSessionChanged(): void {
  authSessionChannel()?.postMessage({ type: 'session-changed' })
}

/** clearPasswordChangeState 清理非敏感的改密拦截状态。 */
function clearPasswordChangeState(): void {
  mustChangePassword = false
  try {
    window.sessionStorage.removeItem(MUST_CHANGE_PASSWORD_KEY)
  } catch (error) {
    console.warn('auth_session_storage_clear_failed', {
      operation: 'clear_password_change_state',
      error: errorDiagnostics(error),
    })
  }
}

/** readPendingEntryPath 恢复本标签页的目标；它仍须通过当前服务端角色校验。 */
function readPendingEntryPath(): string | null {
  if (pendingEntryPath !== null) return pendingEntryPath
  try {
    return window.sessionStorage.getItem(PENDING_ENTRY_PATH_KEY)
  } catch (error) {
    console.warn('auth_session_storage_read_failed', {
      operation: 'read_pending_entry_path', error: errorDiagnostics(error),
    })
    return null
  }
}

function persistEntryPath(entryPath: string): void {
  pendingEntryPath = entryPath
  try {
    window.sessionStorage.setItem(PENDING_ENTRY_PATH_KEY, entryPath)
  } catch (error) {
    console.warn('auth_session_storage_write_failed', {
      operation: 'persist_entry_path', error: errorDiagnostics(error),
    })
  }
}

function clearEntryPath(): void {
  pendingEntryPath = null
  try {
    window.sessionStorage.removeItem(PENDING_ENTRY_PATH_KEY)
  } catch (error) {
    console.warn('auth_session_storage_clear_failed', {
      operation: 'clear_entry_path', error: errorDiagnostics(error),
    })
  }
}

/** rememberCurrentEntryPath 仅暂存受保护的站内地址，不捕获认证页凭据或票据。 */
export function rememberCurrentEntryPath(): void {
  const { pathname, search, hash } = window.location
  const currentPath = `${pathname}${search}${hash}`
  const target = Object.values(ROLE_ROUTES)
    .map((route) => safeInternalPath(currentPath, route.pathPrefix))
    .find(Boolean)
  if (target) persistEntryPath(target)
}

/** persistPasswordChangeState 保留已校验的目标，改密完成前不开放角色页面。 */
function persistPasswordChangeState(entryPath: string): void {
  mustChangePassword = true
  persistEntryPath(entryPath)
  try {
    window.sessionStorage.setItem(MUST_CHANGE_PASSWORD_KEY, 'true')
  } catch (error) {
    console.warn('auth_session_storage_write_failed', {
      operation: 'persist_password_change_state',
      error: errorDiagnostics(error),
    })
  }
}

/**
 * completeLogin 建立内存会话并消费经角色校验的回跳；必须改密时保留目标。
 * refresh token 仍只由后端写入 HttpOnly cookie。
 */
export function completeLogin(response: LoginResponse): string {
  const target = entryPathForRoles(response.account?.roles ?? [], readPendingEntryPath())
  accessToken = response.access_token || null
  if (response.must_change_pwd) {
    persistPasswordChangeState(target)
  } else {
    clearPasswordChangeState()
    clearEntryPath()
  }
  notifyAuthSessionChanged()
  return response.must_change_pwd ? '/auth/change-pwd' : target
}

/** expireLoginSession 清除失效凭据，保留尚未完成的认证回跳供用户重试。 */
export function expireLoginSession(): void {
  accessToken = null
  clearPasswordChangeState()
  notifyAuthSessionChanged()
}

/**
 * clearLoginTokens 清除内存 access token 和非敏感会话状态；HttpOnly cookie 由登出接口删除。
 */
export function clearLoginTokens(): void {
  clearLoginTokensLocally()
  notifyAuthSessionChanged()
}

/** clearLoginTokensLocally 清除收到其他标签会话变更后的本地状态,不再次广播。 */
export function clearLoginTokensLocally(): void {
  accessToken = null
  clearPasswordChangeState()
  clearEntryPath()
}

/** subscribeToAuthSessionChanges 监听其他浏览器标签的会话变更通知。 */
export function subscribeToAuthSessionChanges(listener: () => void): () => void {
  const channel = authSessionChannel()
  if (!channel) return () => undefined
  authSessionListeners.add(listener)
  return () => authSessionListeners.delete(listener)
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

function entryPathForRoles(roles: UserRole[], requestedPath: unknown): string {
  for (const role of roles) {
    const route = roleRouteForRoles([role])
    const target = route && safeInternalPath(requestedPath, route.pathPrefix)
    if (target) return target
  }
  return roleRouteForRoles(roles)?.homePath ?? '/auth/login'
}

/** safeInternalPath 只接受站内绝对路径，阻止登录回跳被构造成外部地址。 */
function safeInternalPath(value: unknown, rolePrefix: string): string | null {
  if (typeof value !== 'string' || /\p{Cc}/u.test(value)) return null
  const target = safeInternalNavigation(value)
  return target?.startsWith(`${rolePrefix}/`) ? target : null
}

/**
 * isPasswordChangeRequired 读取登录时保存的服务端改密要求，供路由边界即时拦截。
 */
export function isPasswordChangeRequired(): boolean {
  if (mustChangePassword) return true
  try {
    mustChangePassword = window.sessionStorage.getItem(MUST_CHANGE_PASSWORD_KEY) === 'true'
    return mustChangePassword
  } catch (error) {
    console.warn('auth_session_storage_read_failed', {
      operation: 'read_password_change_state',
      error: errorDiagnostics(error),
    })
    return false
  }
}

/**
 * completeRequiredPasswordChange 按本次服务端身份校验并消费目标，清除改密拦截。
 */
export function completeRequiredPasswordChange(roles: UserRole[]): string {
  const target = entryPathForRoles(roles, readPendingEntryPath())
  clearPasswordChangeState()
  clearEntryPath()
  return target
}
