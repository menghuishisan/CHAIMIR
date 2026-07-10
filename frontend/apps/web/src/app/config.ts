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

export const appConfig = {
  apiBaseURL: import.meta.env.VITE_API_BASE_URL || window.location.origin,
  wsBaseURL: import.meta.env.VITE_WS_BASE_URL || undefined,
  apiTimeoutMs: parsePositiveInteger(import.meta.env.VITE_API_TIMEOUT_MS, 'VITE_API_TIMEOUT_MS'),
  deploymentMode: parseDeploymentMode(import.meta.env.VITE_DEPLOY_MODE),
} as const

export const platformLayerEnabled = appConfig.deploymentMode === 'saas'
