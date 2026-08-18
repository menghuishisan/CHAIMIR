// config.ts 读取并校验构建期与部署层运行时配置,向应用装配层提供单一配置入口。

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

/**
 * parseSameOriginBaseURL 校验 API 根地址只能落在当前平台 origin。
 * API 客户端会自动携带内存 access token;放行外部 origin 会把令牌交给错误的部署地址。
 * 相对路径是生产部署的标准形式(`/api/v1`),绝对地址仅用于同源的显式配置。
 */
function parseSameOriginBaseURL(value: string | undefined, name: string): string | undefined {
  const normalized = value?.trim()
  if (!normalized) return undefined
  if (normalized.startsWith('//') || normalized.includes('\\')) {
    throw new Error(`${name} 必须使用当前平台的站内地址`)
  }

  let parsed: URL
  try {
    parsed = new URL(normalized, window.location.origin)
  } catch {
    throw new Error(`${name} 必须配置为有效的站内地址`)
  }
  if (
    !['http:', 'https:'].includes(parsed.protocol) ||
    parsed.origin !== window.location.origin ||
    parsed.username ||
    parsed.password ||
    parsed.search ||
    parsed.hash
  ) {
    throw new Error(`${name} 必须使用当前平台的站内地址`)
  }
  return parsed.pathname.replace(/\/+$/, '') || '/'
}

/** parseWebSocketBaseURL 校验 WS 根地址与页面同源,避免票据流向外部站点。 */
function parseWebSocketBaseURL(value: string | undefined, name: string): string | undefined {
  const normalized = value?.trim()
  if (!normalized) return undefined
  if (normalized.startsWith('//') || normalized.includes('\\')) {
    throw new Error(`${name} 必须使用当前平台的站内地址`)
  }

  let parsed: URL
  try {
    parsed = new URL(normalized, window.location.origin)
  } catch {
    throw new Error(`${name} 必须配置为有效的站内地址`)
  }
  const comparableOrigin = parsed.origin
    .replace(/^ws:/, 'http:')
    .replace(/^wss:/, 'https:')
  if (
    !['http:', 'https:', 'ws:', 'wss:'].includes(parsed.protocol) ||
    comparableOrigin !== window.location.origin ||
    parsed.username ||
    parsed.password ||
    parsed.search ||
    parsed.hash
  ) {
    throw new Error(`${name} 必须使用当前平台的站内地址`)
  }
  return normalized.replace(/\/$/, '')
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

export const appConfig = {
  runtime: window.__CHAIMIR_RUNTIME_CONFIG__,
  apiBaseURL: parseSameOriginBaseURL(import.meta.env.VITE_API_BASE_URL, 'VITE_API_BASE_URL') || window.location.origin,
  wsBaseURL: parseWebSocketBaseURL(import.meta.env.VITE_WS_BASE_URL, 'VITE_WS_BASE_URL'),
  sandboxToolOrigin: parseHttpsOrigin(window.__CHAIMIR_RUNTIME_CONFIG__?.sandboxToolOrigin, '沙箱工具 origin'),
  apiTimeoutMs: parsePositiveInteger(import.meta.env.VITE_API_TIMEOUT_MS, 'VITE_API_TIMEOUT_MS'),
  deploymentMode: parseDeploymentMode(window.__CHAIMIR_RUNTIME_CONFIG__?.deploymentMode),
  // 仿真 Worker 单条指令超时:阈值由部署层注入(deploy/config/chaimir.env 同名键),
  // 由装配层读出后传给 sim-sdk 的 SimWorkerClient,不在仿真页里写死数字。
  simWorkerCommandTimeoutMs: parsePositiveInteger(
    import.meta.env.VITE_SIM_WORKER_COMMAND_TIMEOUT_MS,
    'VITE_SIM_WORKER_COMMAND_TIMEOUT_MS',
  ),
  // 仿真自动推进的教学基准节奏(1 倍速的单步间隔);变速档在此基础上按倍数缩放。
  // 仿真工作台与公开回放共用同一个值,保证同一场景两处看起来节奏一致。
  simStepIntervalMs: parsePositiveInteger(
    import.meta.env.VITE_SIM_STEP_INTERVAL_MS,
    'VITE_SIM_STEP_INTERVAL_MS',
  ),
} as const

export const platformLayerEnabled = appConfig.deploymentMode === 'saas'
