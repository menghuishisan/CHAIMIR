// config.ts 读取并校验前端构建期配置，向应用装配层提供单一配置入口。

export type DeploymentMode = 'saas' | 'school'

/**
 * parseDeploymentMode 校验部署形态，避免错误配置意外暴露平台管理入口。
 */
function parseDeploymentMode(value: string | undefined): DeploymentMode {
  const normalized = value?.trim().toLowerCase()
  if (normalized === 'saas' || normalized === 'school') {
    return normalized
  }
  throw new Error('VITE_DEPLOY_MODE 必须配置为 saas 或 school')
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

/** parseHttpsOrigin 校验工具代理使用独立 HTTPS origin，避免同源 iframe 信任边界失效。 */
function parseHttpsOrigin(value: string | undefined, name: string): string {
  const normalized = value?.trim().replace(/\/+$/, '')
  if (!normalized || !/^https:\/\/[^/]+$/.test(normalized)) {
    throw new Error(`${name} 必须配置为 HTTPS origin`)
  }
  return normalized
}

export const appConfig = {
  apiBaseURL: import.meta.env.VITE_API_BASE_URL || window.location.origin,
  wsBaseURL: import.meta.env.VITE_WS_BASE_URL || undefined,
  sandboxToolOrigin: parseHttpsOrigin(import.meta.env.VITE_SANDBOX_TOOL_ORIGIN, 'VITE_SANDBOX_TOOL_ORIGIN'),
  apiTimeoutMs: parsePositiveInteger(import.meta.env.VITE_API_TIMEOUT_MS, 'VITE_API_TIMEOUT_MS'),
  deploymentMode: parseDeploymentMode(import.meta.env.VITE_DEPLOY_MODE),
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
