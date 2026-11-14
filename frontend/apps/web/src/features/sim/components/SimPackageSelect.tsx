// SimPackageSelect 在已上架的仿真场景里按名称搜索并挑一个(实验向导的「仿真场景」字段)。
// 场景目录会越来越多,查找交给 SearchPicker 由服务端完成,不只列第一页;同一场景的多个版本行按编码去重。
// 当前选中场景的名称由调用方给出(调用方已按编码读取了版本列表),本次点选的直接用结果里的名称。

import { useState } from 'react'
import { SIM_PACKAGE_STATUS, type SimPackageMeta } from '@chaimir/api-client'
import { api } from '../../../app/api'
import { SearchPicker, type SearchPickerPage } from '../../../components/SearchPicker'
import { simCategoryLabel } from '../../../utils/labels/sim'

export interface SimPackageSelectProps {
  /** 当前选中的场景编码;空串表示未选 */
  value: string
  /** 当前选中场景的名称(调用方从版本列表读到);还没读到时不传 */
  selectedName?: string
  onChange: (code: string) => void
  /** 触发器的 id,供表单标签关联 */
  id?: string
  /** 无障碍名称 */
  label: string
  invalid?: boolean
}

/** searchPackages 按关键词查已上架的场景;同一编码只留第一行,数量提示按去重后的口径估计 */
async function searchPackages(
  keyword: string,
  limit: number
): Promise<SearchPickerPage<SimPackageMeta>> {
  const page = await api.sim.getPackages({
    keyword: keyword || undefined,
    status: SIM_PACKAGE_STATUS.PUBLISHED,
    page: 1,
    size: limit,
  })
  const seen = new Set<string>()
  const list = page.list.filter((item) => {
    if (seen.has(item.code)) return false
    seen.add(item.code)
    return true
  })
  return { list, total: page.total - (page.list.length - list.length) }
}

/** SimPackageSelect 触发器显示当前选中的场景;点开后按名称查找并点选 */
export function SimPackageSelect({
  value,
  selectedName,
  onChange,
  id,
  label,
  invalid = false,
}: SimPackageSelectProps) {
  const [chosen, setChosen] = useState<SimPackageMeta>()
  const name = chosen && chosen.code === value ? chosen.name : selectedName

  return (
    <SearchPicker<SimPackageMeta>
      id={id}
      label={label}
      triggerText={value ? (name ?? '正在读取…') : '选择场景'}
      empty={!value}
      searchPlaceholder="输入场景名称"
      search={searchPackages}
      itemKey={(item) => item.code}
      renderItem={(item) => (
        <span className="min-w-0 truncate">
          {item.name} · {simCategoryLabel(item.category)}
        </span>
      )}
      isSelected={(item) => item.code === value}
      onSelect={(item) => {
        setChosen(item)
        onChange(item.code)
      }}
      emptyText={(keyword) =>
        keyword ? '没有匹配的仿真场景,换个名称试试。' : '平台还没有已上架的仿真场景。'
      }
      invalid={invalid}
    />
  )
}
