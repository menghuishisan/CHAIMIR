import { API_BASE_PATH } from './http'

/** 内置工具入口须绑定到当前沙箱。 */
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

/** 基础设施只参与组合，不出现在学生工具入口。 */
export const SANDBOX_COMPONENT_CATEGORY = {
  TOOL: 'tool',
  INFRA: 'infra',
} as const

export type SandboxComponentCategory =
  (typeof SANDBOX_COMPONENT_CATEGORY)[keyof typeof SANDBOX_COMPONENT_CATEGORY]

export enum SandboxToolKind {
  BUILTIN = 1,
  TERMINAL = 2,
  WEB_EMBED = 3,
  COMMAND = 4,
  INFRA = 5,
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
