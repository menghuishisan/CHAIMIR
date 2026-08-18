// 内容域题目表格的统一展示单元。
//
// 题库、共享资源库与跨模块选题器都展示相同的题目身份和分类信息;
// 这里统一业务 DTO 到界面文案的映射,基础表格与徽标仍由 @chaimir/ui 提供。

import type { ContentItem } from '@chaimir/api-client'
import { Badge } from '@chaimir/ui'
import { contentDifficultyLabel, contentTypeLabel } from '../../../utils/labels/content'

export interface ContentItemCellProps {
  item: ContentItem
}

/** ContentItemIdentityCell 展示题目名称、稳定编号与锁定版本。 */
export function ContentItemIdentityCell({ item }: ContentItemCellProps) {
  return (
    <div className="min-w-0">
      <div className="truncate font-medium text-ink">{item.title}</div>
      <div className="truncate font-mono text-xs text-ink-sub">
        {item.code} · {item.version}
      </div>
    </div>
  )
}

/** ContentItemClassificationCell 把题型与难度统一映射为内容域徽标。 */
export function ContentItemClassificationCell({ item }: ContentItemCellProps) {
  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <Badge tone="neutral">{contentTypeLabel(item.type)}</Badge>
      <Badge tone="jade">{contentDifficultyLabel(item.difficulty)}</Badge>
    </div>
  )
}
