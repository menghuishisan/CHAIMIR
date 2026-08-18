// 沙箱契约常量：维护前端需要与后端 sandbox/contracts 枚举编号对齐的值。

import { API_BASE_PATH } from './http'

/** 平台内置工具入口模板的固定前缀，与后端 validBuiltinEndpointTemplate 保持一致。 */
export const SANDBOX_BUILTIN_ENDPOINT_TEMPLATE_PREFIX = `${API_BASE_PATH}/sandbox/sandboxes/{sandbox_id}`

export enum SandboxPhase {
  ALLOCATING = 1,
  READY = 2,
  INITIALIZING = 3,
  FULLY_READY = 4,
}

export enum SandboxStatus {
  CREATING = 1,
  RUNNING = 2,
  PAUSED = 3,
  RECYCLING = 4,
  DESTROYED = 5,
  FAILED = 6,
  READY = 7,
  IDLE = 8,
}

export enum SandboxToolKind {
  BUILTIN = 1,
  TERMINAL = 2,
  WEB_EMBED = 3,
  COMMAND = 4,
}

export enum RuntimeAdapterLevel {
  HOSTED = 1,
  STANDARD = 2,
  PLUGIN = 3,
}

export enum RuntimeStatus {
  AVAILABLE = 1,
  ONBOARDING = 2,
  DISABLED = 3,
}

export enum RuntimeSelftestStatus {
  PENDING = 1,
  PASSED = 2,
  FAILED = 3,
}

export enum RuntimeImageStatus {
  AVAILABLE = 1,
  DISABLED = 2,
}

export enum ImagePrepullStatus {
  PENDING = 1,
  SUCCEEDED = 2,
  FAILED = 3,
  RUNNING = 4,
}

export enum ToolStatus {
  AVAILABLE = 1,
  DISABLED = 2,
}

export enum SandboxToolStatus {
  READY = 1,
  STARTING = 2,
  FAILED = 3,
}

/** SandboxChainOperation 是运行时能力声明使用的公开链操作值。 */
export const SANDBOX_CHAIN_OPERATION = {
  DEPLOY: 'deploy',
  TRANSACTION: 'transaction',
  QUERY: 'query',
} as const

export type SandboxChainOperation =
  (typeof SANDBOX_CHAIN_OPERATION)[keyof typeof SANDBOX_CHAIN_OPERATION]
