// sandbox labels 文件维护沙箱、工具、运行时和镜像准备状态文案。

import { ImagePrepullStatus, RuntimeSelftestStatus, RuntimeStatus, SandboxStatus, SandboxToolKind, ToolStatus, type SandboxQuota } from '@chaimir/api-client'
import { labelFromMap } from './map'

type SandboxQuotaField = keyof Omit<SandboxQuota, 'tenant_id' | 'active_sandbox_count'>

/** sandboxQuotaFieldLabels 定义租户沙箱配额字段文案。 */
export const sandboxQuotaFieldLabels: Record<SandboxQuotaField, string> = {
  max_concurrent_sandbox: '最大并发沙箱数', max_cpu: '最大 CPU 毫核', max_memory_mb: '最大内存 MB',
  idle_timeout_min: '空闲回收分钟', max_lifetime_min: '最长运行分钟', max_keepalive_min: '最长保活分钟',
  max_snapshot_retention_min: '快照保留分钟',
}

/** sandboxToolKindLabel 返回沙箱工具类型文案。 */
export function sandboxToolKindLabel(kind: SandboxToolKind): string {
  return labelFromMap(kind, {
    [SandboxToolKind.BUILTIN]: '内置工具', [SandboxToolKind.TERMINAL]: '终端',
    [SandboxToolKind.WEB_EMBED]: '网页工具', [SandboxToolKind.COMMAND]: '受控命令',
  }, '未知')
}

/** toolStatusLabel 返回沙箱工具可用状态文案。 */
export function toolStatusLabel(status: ToolStatus): string {
  return labelFromMap(status, { [ToolStatus.AVAILABLE]: '可用', [ToolStatus.DISABLED]: '已停用' }, '未知')
}

/** runtimeStatusLabel 返回链运行时状态文案。 */
export function runtimeStatusLabel(status: RuntimeStatus): string {
  return labelFromMap(status, {
    [RuntimeStatus.AVAILABLE]: '可用', [RuntimeStatus.ONBOARDING]: '接入中', [RuntimeStatus.DISABLED]: '已停用',
  }, '未知')
}

/** runtimeSelftestStatusLabel 返回链运行时自检状态文案。 */
export function runtimeSelftestStatusLabel(status: RuntimeSelftestStatus): string {
  return labelFromMap(status, {
    [RuntimeSelftestStatus.PENDING]: '待自检', [RuntimeSelftestStatus.PASSED]: '已通过',
    [RuntimeSelftestStatus.FAILED]: '未通过',
  }, '未知')
}

/** sandboxStatusLabel 返回沙箱实例状态文案。 */
export function sandboxStatusLabel(status: SandboxStatus): string {
  return labelFromMap(status, {
    [SandboxStatus.CREATING]: '创建中', [SandboxStatus.RUNNING]: '运行中', [SandboxStatus.PAUSED]: '已暂停',
    [SandboxStatus.RECYCLING]: '回收中', [SandboxStatus.DESTROYED]: '已销毁', [SandboxStatus.FAILED]: '启动失败',
    [SandboxStatus.READY]: '就绪', [SandboxStatus.IDLE]: '空闲',
  }, '未知')
}

/** imagePrepullStatusLabel 返回镜像预拉取状态文案。 */
export function imagePrepullStatusLabel(status: ImagePrepullStatus): string {
  return labelFromMap(status, {
    [ImagePrepullStatus.PENDING]: '等待中', [ImagePrepullStatus.SUCCEEDED]: '已完成',
    [ImagePrepullStatus.FAILED]: '失败', [ImagePrepullStatus.RUNNING]: '进行中',
  }, '未知')
}
