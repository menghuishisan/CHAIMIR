// ContentItemSelect 在已发布题目里按关键词搜索并挑一道题的一个版本(实验检查点、竞赛赛题的「引用题目」字段)。
// 题库可能有成百上千道题,查找交给 SearchPicker 由服务端完成,不只列第一页;
// 限定多个题型时各查一页后合并。当前选中的题目按编码与版本单独读一次标题,编辑已有记录时也能显示是哪道题。
// 只列已发布的题:草稿版本在学生作答与判分时取不到题面,发布实验或开赛时会被拒绝。

import { useState } from 'react'
import { ContentStatus, type ContentItem, type ContentType } from '@chaimir/api-client'
import { api } from '../../../app/api'
import { SearchPicker, type SearchPickerPage } from '../../../components/SearchPicker'
import { useAsyncResource } from '../../../hooks'
import { contentItemVersionLabel } from '../contentItemOptions'

/** 题目的一个锁定版本 */
export interface ContentItemRef {
  code: string
  version: string
}

export interface ContentItemSelectProps {
  /** 当前选中的题目版本;未选时不传 */
  value?: ContentItemRef
  onChange: (item: ContentItemRef) => void
  /** 只在这些题型里找;不传为全部题型 */
  types?: readonly ContentType[]
  /** 触发器的 id,供表单标签关联 */
  id?: string
  /** 无障碍名称 */
  label: string
  invalid?: boolean
  disabled?: boolean
}

/** searchItems 按关键词查已发布题目;多个题型各查一页后合并,总数相加 */
async function searchItems(
  keyword: string,
  limit: number,
  types: readonly ContentType[] | undefined
): Promise<SearchPickerPage<ContentItem>> {
  const base = {
    keyword: keyword || undefined,
    status: ContentStatus.PUBLISHED,
    page: 1,
    size: limit,
  }
  const pages = await Promise.all(
    types && types.length > 0
      ? types.map((type) => api.content.getItems({ ...base, type }))
      : [api.content.getItems(base)]
  )
  return {
    list: pages.flatMap((page) => page.list).slice(0, limit),
    total: pages.reduce((sum, page) => sum + page.total, 0),
  }
}

/** ContentItemSelect 触发器显示当前选中的题目版本;点开后按关键词查找并点选 */
export function ContentItemSelect({
  value,
  onChange,
  types,
  id,
  label,
  invalid = false,
  disabled = false,
}: ContentItemSelectProps) {
  // 本次在结果里点选的题目:标题已知,不必再读
  const [chosen, setChosen] = useState<ContentItem>()
  const known =
    chosen && value && chosen.code === value.code && chosen.version === value.version
      ? chosen
      : undefined
  const picked = useAsyncResource(
    () =>
      value && !known ? api.content.getItemFace(value.code, value.version) : Promise.resolve(null),
    [value?.code, value?.version, Boolean(known)],
    () => false
  )

  /** triggerText 当前选择的说法:标题已知时写全,读不到时退回编码与版本 */
  const triggerText = (): string => {
    if (!value) return '选择题目'
    if (known) return contentItemVersionLabel(known)
    if (picked.status === 'success' && picked.data) return contentItemVersionLabel(picked.data)
    if (picked.status === 'loading') return '正在读取…'
    return `题目编码 ${value.code} · 版本 ${value.version}`
  }

  return (
    <SearchPicker<ContentItem>
      id={id}
      label={label}
      triggerText={triggerText()}
      empty={!value}
      searchPlaceholder="输入题目标题或编码"
      search={(keyword, limit) => searchItems(keyword, limit, types)}
      searchDeps={[types?.join(',')]}
      itemKey={(item) => `${item.code}|${item.version}`}
      renderItem={(item) => (
        <span className="min-w-0 truncate">{contentItemVersionLabel(item)}</span>
      )}
      isSelected={(item) => item.code === value?.code && item.version === value?.version}
      onSelect={(item) => {
        setChosen(item)
        onChange({ code: item.code, version: item.version })
      }}
      emptyText={(keyword) =>
        keyword ? '没有匹配的已发布题目,换个关键词试试。' : '题库里还没有可选的已发布题目。'
      }
      invalid={invalid}
      disabled={disabled}
    />
  )
}
