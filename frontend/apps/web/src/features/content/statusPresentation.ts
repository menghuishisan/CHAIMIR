// content 领域维护状态枚举对应的界面语义色。

import type { StatusTone } from '@chaimir/ui'
import { ContentStatus } from '@chaimir/api-client'

const STATUS_TONES: Record<ContentStatus, StatusTone> = {
  [ContentStatus.DRAFT]: 'neutral',
  [ContentStatus.PUBLISHED]: 'success',
  [ContentStatus.DEPRECATED]: 'warning',
}

/** contentStatusTone 返回内容状态语义色。 */
export function contentStatusTone(status: ContentStatus): StatusTone {
  return STATUS_TONES[status]
}
