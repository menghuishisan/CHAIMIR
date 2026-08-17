/** 浏览器导航安全边界:限制站内跳转和认证离站跳转的可接受地址。 */

/** 只接受当前站点的绝对路径,避免通知链接把用户带到外部站点。 */
export function safeInternalNavigation(value: unknown): string | undefined {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//') || value.includes('\\')) {
    return undefined
  }

  try {
    const parsed = new URL(value, window.location.origin)
    if (parsed.origin !== window.location.origin || parsed.protocol !== window.location.protocol) return undefined
    return `${parsed.pathname}${parsed.search}${parsed.hash}`
  } catch {
    return undefined
  }
}

/** 只接受 HTTPS 认证地址,拒绝降级传输、危险协议和凭据嵌入。 */
export function safeExternalHttpsUrl(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined

  try {
    const parsed = new URL(value)
    if (parsed.protocol !== 'https:' || parsed.username || parsed.password) return undefined
    return parsed.toString()
  } catch {
    return undefined
  }
}

/** safeMonitoringUrl 校验运维配置的监控入口:仅 HTTPS origin + path,不携带凭据或令牌参数。 */
export function safeMonitoringUrl(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined
  try {
    const parsed = new URL(value.trim())
    if (parsed.protocol !== 'https:' || parsed.username || parsed.password || parsed.search || parsed.hash) {
      return undefined
    }
    return parsed.href
  } catch {
    return undefined
  }
}
