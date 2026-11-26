export type DeploymentMode = 'saas' | 'school'

/**
 * parseDeploymentMode 校验部署形态，避免错误配置意外暴露平台管理入口。
 */
function parseDeploymentMode(value: string | undefined): DeploymentMode {
  const normalized = value?.trim().toLowerCase()
  if (normalized === 'saas' || normalized === 'school') {
    return normalized
  }
  throw new Error('部署运行时配置缺失或无效,请联系管理员')
}

/**
 * parsePositiveInteger 校验正整数型前端配置，阻止无效超时进入请求层。
 */
function parsePositiveInteger(value: string | undefined, name: string): number {
  const parsed = Number(value)
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`${name} 必须配置为正整数`)
  }
  return parsed
}

// 请求会自动携带会话凭据，API 与 WebSocket 根路径都必须属于当前站点。
function sameOriginPath(value: string, name: string, webSocket = false): string {
  const normalized = value.trim()
  const invalid = `${name} 必须使用当前平台的站内地址`
  if (!normalized || normalized.startsWith('//') || normalized.includes('\\')) {
    throw new Error(invalid)
  }

  let parsed: URL
  try {
    parsed = new URL(normalized, window.location.origin)
  } catch {
    throw new Error(invalid)
  }
  const origin = webSocket
    ? parsed.origin.replace(/^ws:/, 'http:').replace(/^wss:/, 'https:')
    : parsed.origin
  const protocols = webSocket ? ['http:', 'https:', 'ws:', 'wss:'] : ['http:', 'https:']
  if (
    !protocols.includes(parsed.protocol) ||
    origin !== window.location.origin ||
    parsed.username ||
    parsed.password ||
    parsed.search ||
    parsed.hash
  ) {
    throw new Error(invalid)
  }
  return parsed.pathname.replace(/\/+$/, '') || '/'
}

/** parseHttpsOrigin 校验工具代理使用独立 HTTPS origin，避免同源 iframe 信任边界失效。 */
function parseHttpsOrigin(value: string | undefined, name: string): string {
  let parsed: URL
  try {
    parsed = new URL(value?.trim() || '')
  } catch {
    throw new Error(`${name} 必须配置为 HTTPS origin`)
  }
  if (
    parsed.protocol !== 'https:' ||
    parsed.username ||
    parsed.password ||
    parsed.pathname !== '/' ||
    parsed.search ||
    parsed.hash
  ) {
    throw new Error(`${name} 必须配置为 HTTPS origin`)
  }
  if (parsed.origin === window.location.origin) {
    throw new Error(`${name} 必须使用独立于平台的工具 origin`)
  }
  return parsed.origin
}

const runtime = window.__CHAIMIR_RUNTIME_CONFIG__
const apiBaseURL = sameOriginPath(import.meta.env.VITE_API_BASE_URL ?? '', 'VITE_API_BASE_URL')
if (apiBaseURL === '/') throw new Error('VITE_API_BASE_URL 必须指定 API 路径')
const configuredWsBaseURL = import.meta.env.VITE_WS_BASE_URL?.trim()

export const appConfig = {
  apiBaseURL,
  wsBaseURL: configuredWsBaseURL && sameOriginPath(configuredWsBaseURL, 'VITE_WS_BASE_URL', true),
  sandboxToolOrigin: parseHttpsOrigin(runtime?.sandboxToolOrigin, '沙箱工具 origin'),
  apiTimeoutMs: parsePositiveInteger(import.meta.env.VITE_API_TIMEOUT_MS, 'VITE_API_TIMEOUT_MS'),
  runtimeSelftestPollIntervalMs: parsePositiveInteger(
    import.meta.env.VITE_RUNTIME_SELFTEST_POLL_INTERVAL_MS,
    'VITE_RUNTIME_SELFTEST_POLL_INTERVAL_MS',
  ),
  runtimeSelftestPollTimeoutMs: parsePositiveInteger(
    import.meta.env.VITE_RUNTIME_SELFTEST_POLL_TIMEOUT_MS,
    'VITE_RUNTIME_SELFTEST_POLL_TIMEOUT_MS',
  ),
  deploymentMode: parseDeploymentMode(runtime?.deploymentMode),
  simWorkerCommandTimeoutMs: parsePositiveInteger(
    import.meta.env.VITE_SIM_WORKER_COMMAND_TIMEOUT_MS,
    'VITE_SIM_WORKER_COMMAND_TIMEOUT_MS',
  ),
  // 工作台与公开回放共用 1 倍速的单步间隔。
  simStepIntervalMs: parsePositiveInteger(
    import.meta.env.VITE_SIM_STEP_INTERVAL_MS,
    'VITE_SIM_STEP_INTERVAL_MS',
  ),
} as const

export const platformLayerEnabled = appConfig.deploymentMode === 'saas'
