// 浏览器存储：集中管理四端登录态与 trace_id，避免各端散落 key。

const ACCESS_TOKEN_KEY = 'chaimir.access_token'
const REFRESH_TOKEN_KEY = 'chaimir.refresh_token'
const TRACE_ID_KEY = 'chaimir.trace_id'
const USER_INFO_KEY = 'chaimir.user_info'

/**
 * getAccessToken 读取后端访问令牌，未登录时返回 null。
 */
export function getAccessToken(): string | null {
  return safeRead(ACCESS_TOKEN_KEY)
}

/**
 * saveSession 保存后端签发的登录态，供四端 API 客户端统一读取。
 */
export function saveSession(accessToken?: string, refreshToken?: string): void {
  if (accessToken) {
    safeWrite(ACCESS_TOKEN_KEY, accessToken)
  }
  if (refreshToken) {
    safeWrite(REFRESH_TOKEN_KEY, refreshToken)
  }
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event('chaimir-auth-change'))
  }
}

/**
 * getRefreshToken 读取后端刷新令牌，供认证上下文执行服务端轮转。
 */
export function getRefreshToken(): string | null {
  return safeRead(REFRESH_TOKEN_KEY)
}

/**
 * getStoredUser 读取当前浏览器缓存的用户信息，缓存损坏时返回 null。
 */
export function getStoredUser<T>(): T | null {
  const raw = safeRead(USER_INFO_KEY)
  if (!raw) {
    return null
  }
  try {
    return JSON.parse(raw) as T
  } catch (error) {
    console.warn('用户缓存已损坏，已清理本地缓存', error)
    safeRemove(USER_INFO_KEY)
    return null
  }
}

/**
 * saveStoredUser 保存用户资料缓存，认证判定仍以 access token 为准。
 */
export function saveStoredUser(user: unknown): void {
  safeWrite(USER_INFO_KEY, JSON.stringify(user))
}

/**
 * clearSession 清除当前浏览器登录态，用于登出或鉴权失效后的显式收敛。
 */
export function clearSession(): void {
  safeRemove(ACCESS_TOKEN_KEY)
  safeRemove(REFRESH_TOKEN_KEY)
  safeRemove(USER_INFO_KEY)
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new Event('chaimir-auth-change'))
  }
}

/**
 * getTraceId 读取当前前端会话 trace_id，用于请求链路关联。
 */
export function getTraceId(): string | null {
  const existing = safeRead(TRACE_ID_KEY)
  if (existing) {
    return existing
  }
  const generated = `fe-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
  safeWrite(TRACE_ID_KEY, generated)
  return generated
}

/**
 * safeRead 防止隐私模式或受限浏览器环境导致页面白屏。
 */
function safeRead(key: string): string | null {
  if (typeof window === 'undefined') {
    return null
  }
  try {
    return window.localStorage.getItem(key)
  } catch (error) {
    reportStorageError('读取浏览器存储失败', error)
    return null
  }
}

/**
 * safeWrite 在存储不可用时显式兜底，不影响页面继续渲染。
 */
function safeWrite(key: string, value: string): void {
  if (typeof window === 'undefined') {
    return
  }
  try {
    window.localStorage.setItem(key, value)
  } catch (error) {
    reportStorageError('写入本地存储失败，尝试写入会话存储', error)
    try {
      window.sessionStorage.setItem(key, value)
    } catch (sessionError) {
      reportStorageError('写入会话存储失败', sessionError)
      return
    }
  }
}

/**
 * safeRemove 在存储受限时也不让清理动作影响页面可用性。
 */
function safeRemove(key: string): void {
  if (typeof window === 'undefined') {
    return
  }
  try {
    window.localStorage.removeItem(key)
    window.sessionStorage.removeItem(key)
  } catch (error) {
    reportStorageError('清理浏览器存储失败', error)
    return
  }
}

/**
 * reportStorageError 仅向开发控制台记录浏览器存储异常，页面不展示内部细节。
 */
function reportStorageError(message: string, error: unknown): void {
  console.warn(message, error)
}
