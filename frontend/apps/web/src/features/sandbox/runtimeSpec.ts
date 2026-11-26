// 从管理员声明中读取运行时摘要；工作负载模型由后端维护。

import { asRecord, stringValue } from './dynamicSpec'

/** runtimeWorkspaceDir 返回运行时工作区目录,缺失时给出空值供界面展示。 */
export function runtimeWorkspaceDir(spec: Record<string, unknown>): string {
  return stringValue(spec.workspace_dir)
}

/** runtimeContainerName 返回主环境名称。 */
export function runtimeContainerName(spec: Record<string, unknown>): string {
  return stringValue(asRecord(spec.runtime_container)?.name)
}

/** runtimeContainerPortCount 返回主环境声明的端口数量。 */
export function runtimeContainerPortCount(spec: Record<string, unknown>): number {
  const ports = asRecord(spec.runtime_container)?.ports
  return Array.isArray(ports) ? ports.length : 0
}

/** runtimeSidecarCount 返回附加组件数量。 */
export function runtimeSidecarCount(spec: Record<string, unknown>): number {
  return Array.isArray(spec.infra_sidecars) ? spec.infra_sidecars.length : 0
}

/** runtimeHasDeployCommand 判断声明中是否存在部署命令。 */
export function runtimeHasDeployCommand(spec: Record<string, unknown>): boolean {
  const command = asRecord(asRecord(spec.capability_commands)?.deploy)?.command
  return Array.isArray(command) && command.length > 0
}

/**
 * runtimeDisabledReason 返回服务端写入的不可部署原因。
 * 它由平台目录同步产出(镜像缺证明、manifest 声明不可部署、尚未完成自检等),
 * 是用户向说明,页面原样展示 —— 前端不自己推断也不隐藏(对齐清单 §6.3 / §8.3)。
 */
export function runtimeDisabledReason(spec: Record<string, unknown>): string {
  return stringValue(spec.disabled_reason)
}
