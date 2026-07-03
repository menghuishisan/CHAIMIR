// 浏览器存储：集中管理四端登录态与 trace_id，避免各端散落 key。

const ACCESS_TOKEN_KEY = 'chaimir.access_token'
const REFRESH_TOKEN_KEY = 'chaimir.refresh_token'
const TRACE_ID_KEY = 'chaimir.trace_id'

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
 * clearSession 清除当前浏览器登录态，用于登出或鉴权失效后的显式收敛。
 */
export function clearSession(): void {
  safeRemove(ACCESS_TOKEN_KEY)
  safeRemove(REFRESH_TOKEN_KEY)
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
  } catch {
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
  } catch {
    try {
      window.sessionStorage.setItem(key, value)
    } catch {
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
  } catch {
    return
  }
}
