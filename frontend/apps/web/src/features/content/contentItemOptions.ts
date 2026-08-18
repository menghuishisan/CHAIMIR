// contentItemOptions 把内容域题目版本转换成选择控件需要的稳定选项。
// 竞赛题目和实验检查点都引用同一份锁定题目版本,展示口径由内容域统一维护。

import type { ContentItem } from '@chaimir/api-client'
import { contentTypeLabel } from '../../utils/labels/content'

export interface ContentItemOption {
  value: string
  label: string
}

/** toContentItemVersionOptions 为题目版本生成 code|version 选项。 */
export function toContentItemVersionOptions(items: ContentItem[]): ContentItemOption[] {
  return items.map((item) => ({
    value: `${item.code}|${item.version}`,
    label: `${item.title} · ${contentTypeLabel(item.type)} · ${item.version}`,
  }))
}
