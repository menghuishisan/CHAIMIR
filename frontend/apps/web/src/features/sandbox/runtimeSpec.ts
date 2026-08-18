// runtimeSpec 负责读取沙箱运行时声明中的公开摘要字段。
// 声明本体是平台管理员提交的动态 JSON,不在应用层复制后端 workload 编排模型。

/** asRecord 将未知 JSON 值收敛为对象,非对象由调用方按缺省处理。 */
export function asRecord(value: unknown): Record<string, unknown> | undefined {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : undefined
}

/** readString 读取动态声明中的非空字符串字段。 */
export function readString(record: Record<string, unknown> | undefined, key: string): string {
  const value = record?.[key]
  return typeof value === 'string' ? value : ''
}

/** runtimeWorkspaceDir 返回运行时工作区目录,缺失时给出空值供界面展示。 */
export function runtimeWorkspaceDir(spec: Record<string, unknown>): string {
  return readString(spec, 'workspace_dir')
}

/** runtimeContainerName 返回主环境名称。 */
export function runtimeContainerName(spec: Record<string, unknown>): string {
  return readString(asRecord(spec.runtime_container), 'name')
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

/** runtimeDefaultToolCodes 返回默认工具编码列表。 */
export function runtimeDefaultToolCodes(spec: Record<string, unknown>): string[] {
  return Array.isArray(spec.default_tool_codes)
    ? spec.default_tool_codes.filter((item): item is string => typeof item === 'string')
    : []
}

/** runtimeHasDeployCommand 判断声明中是否存在部署命令。 */
export function runtimeHasDeployCommand(spec: Record<string, unknown>): boolean {
  const command = asRecord(asRecord(spec.capability_commands)?.deploy)?.command
  return Array.isArray(command) && command.length > 0
}
