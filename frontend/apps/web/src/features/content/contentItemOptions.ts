// contentItemOptions 题目版本的统一说法:竞赛赛题和实验检查点都引用同一份锁定题目版本,展示口径由内容域维护。

import type { ContentItem } from '@chaimir/api-client'
import { contentTypeLabel } from '../../utils/labels/content'

/** contentItemVersionLabel 一道题的一个版本写成一行:标题 · 题型 · 版本 */
export function contentItemVersionLabel(
  item: Pick<ContentItem, 'title' | 'type' | 'version'>
): string {
  return `${item.title} · ${contentTypeLabel(item.type)} · ${item.version}`
}
