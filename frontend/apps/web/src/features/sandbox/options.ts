// sandbox options 文件维护 M2 管理表单与进度组件使用的封闭选项顺序。

import { SandboxPhase, SandboxStatus, SandboxToolKind, ToolStatus } from '@chaimir/api-client'

/** SANDBOX_PHASES 按实验环境准备顺序供进度组件渲染。 */
export const SANDBOX_PHASES = [
  SandboxPhase.ALLOCATING,
  SandboxPhase.READY,
  SandboxPhase.INITIALIZING,
  SandboxPhase.FULLY_READY,
] as const

/** SANDBOX_STATUSES 是进度推送允许出现的封闭运行状态集合。 */
export const SANDBOX_STATUSES = [
  SandboxStatus.CREATING,
  SandboxStatus.RUNNING,
  SandboxStatus.PAUSED,
  SandboxStatus.RECYCLING,
  SandboxStatus.DESTROYED,
  SandboxStatus.FAILED,
  SandboxStatus.READY,
  SandboxStatus.IDLE,
] as const

/** SANDBOX_TOOL_KINDS 按登记顺序供工具表单渲染。 */
export const SANDBOX_TOOL_KINDS = [
  SandboxToolKind.BUILTIN,
  SandboxToolKind.TERMINAL,
  SandboxToolKind.WEB_EMBED,
  SandboxToolKind.COMMAND,
] as const

/** TOOL_STATUSES 供工具表单渲染可选状态。 */
export const TOOL_STATUSES = [ToolStatus.AVAILABLE, ToolStatus.DISABLED] as const
