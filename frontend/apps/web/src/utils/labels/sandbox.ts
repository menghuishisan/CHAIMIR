// sandbox labels 文件维护 M2 沙箱引擎枚举的用户向文案。

import {
  ImagePrepullStatus,
  RuntimeImageStatus,
  RuntimeSelftestStatus,
  RuntimeStatus,
  SandboxPhase,
  SandboxToolKind,
  type SandboxChainOperation,
  SANDBOX_CHAIN_OPERATION,
  ToolStatus,
} from '@chaimir/api-client'

const SANDBOX_CHAIN_OPERATION_LABELS: Record<SandboxChainOperation, string> = {
  [SANDBOX_CHAIN_OPERATION.DEPLOY]: '部署合约',
  [SANDBOX_CHAIN_OPERATION.TRANSACTION]: '发起交易',
  [SANDBOX_CHAIN_OPERATION.QUERY]: '查询链上状态',
}

/** sandboxChainOperationLabel 返回运行时声明的链操作用户向文案。 */
export function sandboxChainOperationLabel(operation: SandboxChainOperation): string {
  return SANDBOX_CHAIN_OPERATION_LABELS[operation]
}

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

const RUNTIME_STATUS_LABELS: Record<RuntimeStatus, string> = {
  [RuntimeStatus.AVAILABLE]: '可用',
  [RuntimeStatus.ONBOARDING]: '接入中',
  [RuntimeStatus.DISABLED]: '已停用',
}

/** runtimeStatusLabel 返回链运行时状态文案。 */
export function runtimeStatusLabel(status: RuntimeStatus): string {
  return RUNTIME_STATUS_LABELS[status]
}

const SELFTEST_STATUS_LABELS: Record<RuntimeSelftestStatus, string> = {
  [RuntimeSelftestStatus.PENDING]: '尚未自检',
  [RuntimeSelftestStatus.PASSED]: '自检通过',
  [RuntimeSelftestStatus.FAILED]: '自检未通过',
}

/** runtimeSelftestStatusLabel 返回运行时自检状态文案。 */
export function runtimeSelftestStatusLabel(status: RuntimeSelftestStatus): string {
  return SELFTEST_STATUS_LABELS[status]
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

/** imagePrepullStatusLabel 返回镜像预拉取状态文案。 */
export function imagePrepullStatusLabel(status: ImagePrepullStatus): string {
  return PREPULL_STATUS_LABELS[status]
}

const DEFINITION_STATUS_LABELS: Record<ToolStatus, string> = {
  [ToolStatus.AVAILABLE]: '可用',
  [ToolStatus.DISABLED]: '已停用',
}

/** toolStatusLabel 返回工具定义状态文案。 */
export function toolStatusLabel(status: ToolStatus): string {
  return DEFINITION_STATUS_LABELS[status]
}


/**
 * ecoTagsLabel 返回工具适用生态的用户向文案。
 * 后端用 `*` 表示不限生态(如受控终端),直接把星号显示给管理员是内部记法泄漏(FE-4);
 * 空数组同样按不限生态呈现 —— 两者对使用者是同一件事。
 */
export function ecoTagsLabel(ecoTags: string[]): string {
  if (ecoTags.length === 0 || ecoTags.includes('*')) return '不限生态'
  return ecoTags.join('、')
}
