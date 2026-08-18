// sandbox statusPresentation 文件维护 M2 状态到设计系统语义色的映射。

import type { StatusTone } from '@chaimir/ui'
import { ImagePrepullStatus, RuntimeSelftestStatus, RuntimeStatus, ToolStatus } from '@chaimir/api-client'

const RUNTIME_STATUS_TONES: Record<RuntimeStatus, StatusTone> = {
  [RuntimeStatus.AVAILABLE]: 'success',
  [RuntimeStatus.ONBOARDING]: 'warning',
  [RuntimeStatus.DISABLED]: 'neutral',
}

/** runtimeStatusTone 返回链运行时状态语义色。 */
export function runtimeStatusTone(status: RuntimeStatus): StatusTone {
  return RUNTIME_STATUS_TONES[status]
}

const SELFTEST_STATUS_TONES: Record<RuntimeSelftestStatus, StatusTone> = {
  [RuntimeSelftestStatus.PENDING]: 'neutral',
  [RuntimeSelftestStatus.PASSED]: 'success',
  [RuntimeSelftestStatus.FAILED]: 'danger',
}

/** runtimeSelftestStatusTone 返回运行时自检状态语义色。 */
export function runtimeSelftestStatusTone(status: RuntimeSelftestStatus): StatusTone {
  return SELFTEST_STATUS_TONES[status]
}

const PREPULL_STATUS_TONES: Record<ImagePrepullStatus, StatusTone> = {
  [ImagePrepullStatus.PENDING]: 'neutral',
  [ImagePrepullStatus.RUNNING]: 'info',
  [ImagePrepullStatus.SUCCEEDED]: 'success',
  [ImagePrepullStatus.FAILED]: 'danger',
}

/** imagePrepullStatusTone 返回镜像预拉取状态语义色。 */
export function imagePrepullStatusTone(status: ImagePrepullStatus): StatusTone {
  return PREPULL_STATUS_TONES[status]
}

const TOOL_STATUS_TONES: Record<ToolStatus, StatusTone> = {
  [ToolStatus.AVAILABLE]: 'success',
  [ToolStatus.DISABLED]: 'neutral',
}

/** toolStatusTone 返回工具定义状态语义色。 */
export function toolStatusTone(status: ToolStatus): StatusTone {
  return TOOL_STATUS_TONES[status]
}
