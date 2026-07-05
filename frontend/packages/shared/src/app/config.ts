// 前端应用配置：集中读取四端构建期环境变量，避免各端重复解析。

import { API_BASE_PATH } from '@chaimir/api-client'

export interface FrontendRuntimeConfig {
  apiBaseUrl: string
  wsBaseUrl?: string
  deployMode: 'saas' | 'school'
  requestTimeoutMs: number
  simWorkerCommandTimeoutMs: number
  roleAppUrls: {
    student: string
    teacher: string
    schoolAdmin: string
    platformAdmin: string
  }
}

type EnvMap = Record<string, string | undefined>

/**
 * readFrontendConfig 从构建期环境变量读取运行配置，并提供同源默认值。
 */
export function readFrontendConfig(env: EnvMap = readImportMetaEnv()): FrontendRuntimeConfig {
  return {
    apiBaseUrl: env.VITE_API_BASE_URL || API_BASE_PATH,
    wsBaseUrl: env.VITE_WS_BASE_URL,
    deployMode: readDeployMode(env.VITE_DEPLOY_MODE),
    requestTimeoutMs: readNumber(env.VITE_API_TIMEOUT_MS, 30000),
    simWorkerCommandTimeoutMs: readNumber(env.VITE_SIM_WORKER_COMMAND_TIMEOUT_MS, 2000),
    roleAppUrls: {
      student: env.VITE_STUDENT_APP_URL || '/student/',
      teacher: env.VITE_TEACHER_APP_URL || '/teacher/',
      schoolAdmin: env.VITE_SCHOOL_ADMIN_APP_URL || '/school-admin/',
      platformAdmin: env.VITE_PLATFORM_ADMIN_APP_URL || '/platform-admin/',
    },
  }
}

/**
 * readDeployMode 读取前端部署形态，私有化部署用于关闭平台层公共入口。
 */
function readDeployMode(value: string | undefined): FrontendRuntimeConfig['deployMode'] {
  return value === 'school' ? 'school' : 'saas'
}

/**
 * readImportMetaEnv 兼容浏览器构建环境与普通 TypeScript 编译环境。
 */
function readImportMetaEnv(): EnvMap {
  const meta = import.meta as ImportMeta & { env?: EnvMap }
  return meta.env ?? {}
}

/**
 * readNumber 只接受正整数环境变量，非法值统一回落到文档定义的默认行为。
 */
function readNumber(value: string | undefined, fallback: number): number {
  if (!value) {
    return fallback
  }
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}
