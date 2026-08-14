// sandbox labels 文件维护 M2 沙箱引擎枚举的用户向文案与语义色。

import type { StatusTone } from '@chaimir/ui'
import {
  ImagePrepullStatus,
  RuntimeImageStatus,
  RuntimeSelftestStatus,
  RuntimeStatus,
  SandboxPhase,
  SandboxStatus,
  SandboxToolKind,
  SandboxToolStatus,
  ToolStatus,
} from '@chaimir/api-client'

const SANDBOX_PHASE_LABELS: Record<SandboxPhase, string> = {
  [SandboxPhase.ALLOCATING]: '正在分配资源',
  [SandboxPhase.READY]: '环境已就绪',
  [SandboxPhase.INITIALIZING]: '正在初始化环境',
  [SandboxPhase.FULLY_READY]: '环境已就绪',
}

/** sandboxPhaseLabel 返回实验环境准备阶段文案(区块孵化器的阶段名)。 */
export function sandboxPhaseLabel(phase: SandboxPhase): string {
  return SANDBOX_PHASE_LABELS[phase]
}

/** SANDBOX_PHASES 按准备顺序排列,供 ChainProgress 表达准备进度。 */
export const SANDBOX_PHASES = [
  SandboxPhase.ALLOCATING,
  SandboxPhase.READY,
  SandboxPhase.INITIALIZING,
  SandboxPhase.FULLY_READY,
] as const

const SANDBOX_STATUS_LABELS: Record<SandboxStatus, string> = {
  [SandboxStatus.CREATING]: '正在创建',
  [SandboxStatus.RUNNING]: '运行中',
  [SandboxStatus.PAUSED]: '已暂停',
  [SandboxStatus.RECYCLING]: '正在回收',
  [SandboxStatus.DESTROYED]: '已销毁',
  [SandboxStatus.FAILED]: '创建失败',
  [SandboxStatus.READY]: '已就绪',
  [SandboxStatus.IDLE]: '空闲中',
}

const SANDBOX_STATUS_TONES: Record<SandboxStatus, StatusTone> = {
  [SandboxStatus.CREATING]: 'info',
  [SandboxStatus.RUNNING]: 'primary',
  [SandboxStatus.PAUSED]: 'warning',
  [SandboxStatus.RECYCLING]: 'neutral',
  [SandboxStatus.DESTROYED]: 'neutral',
  [SandboxStatus.FAILED]: 'danger',
  [SandboxStatus.READY]: 'success',
  [SandboxStatus.IDLE]: 'neutral',
}

/** sandboxStatusLabel 返回沙箱运行状态文案。 */
export function sandboxStatusLabel(status: SandboxStatus): string {
  return SANDBOX_STATUS_LABELS[status]
}

/** sandboxStatusTone 返回沙箱运行状态语义色。 */
export function sandboxStatusTone(status: SandboxStatus): StatusTone {
  return SANDBOX_STATUS_TONES[status]
}

const TOOL_KIND_LABELS: Record<SandboxToolKind, string> = {
  [SandboxToolKind.BUILTIN]: '内置能力',
  [SandboxToolKind.TERMINAL]: '终端',
  [SandboxToolKind.WEB_EMBED]: '网页工具',
  [SandboxToolKind.COMMAND]: '命令工具',
}

/** sandboxToolKindLabel 返回沙箱工具形态文案。 */
export function sandboxToolKindLabel(kind: SandboxToolKind): string {
  return TOOL_KIND_LABELS[kind]
}

/** SANDBOX_TOOL_KINDS 供表单按登记顺序渲染工具形态选项。 */
export const SANDBOX_TOOL_KINDS = [
  SandboxToolKind.BUILTIN,
  SandboxToolKind.TERMINAL,
  SandboxToolKind.WEB_EMBED,
  SandboxToolKind.COMMAND,
] as const

const TOOL_STATUS_LABELS: Record<SandboxToolStatus, string> = {
  [SandboxToolStatus.READY]: '可用',
  [SandboxToolStatus.STARTING]: '正在启动',
  [SandboxToolStatus.FAILED]: '启动失败',
}

const TOOL_STATUS_TONES: Record<SandboxToolStatus, StatusTone> = {
  [SandboxToolStatus.READY]: 'success',
  [SandboxToolStatus.STARTING]: 'info',
  [SandboxToolStatus.FAILED]: 'danger',
}

/** sandboxToolStatusLabel 返回工具实例状态文案。 */
export function sandboxToolStatusLabel(status: SandboxToolStatus): string {
  return TOOL_STATUS_LABELS[status]
}

/** sandboxToolStatusTone 返回工具实例状态语义色。 */
export function sandboxToolStatusTone(status: SandboxToolStatus): StatusTone {
  return TOOL_STATUS_TONES[status]
}

const RUNTIME_STATUS_LABELS: Record<RuntimeStatus, string> = {
  [RuntimeStatus.AVAILABLE]: '可用',
  [RuntimeStatus.ONBOARDING]: '接入中',
  [RuntimeStatus.DISABLED]: '已停用',
}

const RUNTIME_STATUS_TONES: Record<RuntimeStatus, StatusTone> = {
  [RuntimeStatus.AVAILABLE]: 'success',
  [RuntimeStatus.ONBOARDING]: 'warning',
  [RuntimeStatus.DISABLED]: 'neutral',
}

/** runtimeStatusLabel 返回链运行时状态文案。 */
export function runtimeStatusLabel(status: RuntimeStatus): string {
  return RUNTIME_STATUS_LABELS[status]
}

/** runtimeStatusTone 返回链运行时状态语义色。 */
export function runtimeStatusTone(status: RuntimeStatus): StatusTone {
  return RUNTIME_STATUS_TONES[status]
}

/** RUNTIME_STATUSES 供表单渲染运行时状态选项。 */
export const RUNTIME_STATUSES = [
  RuntimeStatus.AVAILABLE,
  RuntimeStatus.ONBOARDING,
  RuntimeStatus.DISABLED,
] as const

const SELFTEST_STATUS_LABELS: Record<RuntimeSelftestStatus, string> = {
  [RuntimeSelftestStatus.PENDING]: '尚未自检',
  [RuntimeSelftestStatus.PASSED]: '自检通过',
  [RuntimeSelftestStatus.FAILED]: '自检未通过',
}

const SELFTEST_STATUS_TONES: Record<RuntimeSelftestStatus, StatusTone> = {
  [RuntimeSelftestStatus.PENDING]: 'neutral',
  [RuntimeSelftestStatus.PASSED]: 'success',
  [RuntimeSelftestStatus.FAILED]: 'danger',
}

/** runtimeSelftestStatusLabel 返回运行时自检状态文案。 */
export function runtimeSelftestStatusLabel(status: RuntimeSelftestStatus): string {
  return SELFTEST_STATUS_LABELS[status]
}

/** runtimeSelftestStatusTone 返回运行时自检状态语义色。 */
export function runtimeSelftestStatusTone(status: RuntimeSelftestStatus): StatusTone {
  return SELFTEST_STATUS_TONES[status]
}

const IMAGE_STATUS_LABELS: Record<RuntimeImageStatus, string> = {
  [RuntimeImageStatus.AVAILABLE]: '可用',
  [RuntimeImageStatus.DISABLED]: '已停用',
}

/** runtimeImageStatusLabel 返回镜像状态文案。 */
export function runtimeImageStatusLabel(status: RuntimeImageStatus): string {
  return IMAGE_STATUS_LABELS[status]
}

const PREPULL_STATUS_LABELS: Record<ImagePrepullStatus, string> = {
  [ImagePrepullStatus.PENDING]: '等待预拉取',
  [ImagePrepullStatus.RUNNING]: '正在预拉取',
  [ImagePrepullStatus.SUCCEEDED]: '全部节点已就绪',
  [ImagePrepullStatus.FAILED]: '预拉取失败',
}

const PREPULL_STATUS_TONES: Record<ImagePrepullStatus, StatusTone> = {
  [ImagePrepullStatus.PENDING]: 'neutral',
  [ImagePrepullStatus.RUNNING]: 'info',
  [ImagePrepullStatus.SUCCEEDED]: 'success',
  [ImagePrepullStatus.FAILED]: 'danger',
}

/** imagePrepullStatusLabel 返回镜像预拉取状态文案。 */
export function imagePrepullStatusLabel(status: ImagePrepullStatus): string {
  return PREPULL_STATUS_LABELS[status]
}

/** imagePrepullStatusTone 返回镜像预拉取状态语义色。 */
export function imagePrepullStatusTone(status: ImagePrepullStatus): StatusTone {
  return PREPULL_STATUS_TONES[status]
}

const DEFINITION_STATUS_LABELS: Record<ToolStatus, string> = {
  [ToolStatus.AVAILABLE]: '可用',
  [ToolStatus.DISABLED]: '已停用',
}

const DEFINITION_STATUS_TONES: Record<ToolStatus, StatusTone> = {
  [ToolStatus.AVAILABLE]: 'success',
  [ToolStatus.DISABLED]: 'neutral',
}

/** toolStatusLabel 返回工具定义状态文案。 */
export function toolStatusLabel(status: ToolStatus): string {
  return DEFINITION_STATUS_LABELS[status]
}

/** toolStatusTone 返回工具定义状态语义色。 */
export function toolStatusTone(status: ToolStatus): StatusTone {
  return DEFINITION_STATUS_TONES[status]
}

/** TOOL_STATUSES 供表单渲染工具定义状态选项。 */
export const TOOL_STATUSES = [ToolStatus.AVAILABLE, ToolStatus.DISABLED] as const
