// 实验工具(平台侧栏,/platform-admin/sandbox-tools)。设计 §7.6:
// 页头 +「登记工具」;分段按类别(学生工具 / 基础设施 / 全部,带数字,写进地址);
// 列表行显示名称、类型、是否可部署、生态,不可部署的直接写原因;整行打开工具大抽屉(详情与修改同一处)。
// 不再使用的工具只停用不删除(抽屉里说明)。指标条、平铺全部属性的卡片和常驻说明卡去掉。
// 工具接口一次返回全部(不分页),计数在本页完成。

import { useMemo, useState } from 'react'
import { Package, Plus } from 'lucide-react'
import {
  SANDBOX_COMPONENT_CATEGORY,
  ToolStatus,
  type SandboxToolDefinition,
} from '@chaimir/api-client'
import {
  Button,
  ListRow,
  PageHeader,
  PageScaffold,
  SegmentedControl,
  Skeleton,
  StatusIndicator,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, useUrlState } from '../../../../hooks'
import {
  ecoTagsLabel,
  sandboxToolKindLabel,
  toolStatusLabel,
} from '../../../../utils/labels/sandbox'
import { toolStatusTone } from '../../statusPresentation'
import { ToolFormModal } from './tool-form'

/** 地址参数:category 分段(默认学生工具) */
const TOOLS_QUERY = {
  category: { defaultValue: 'tool', allowed: ['tool', 'infra', 'all'] },
} as const

const CATEGORY_SEGMENTS = [
  { value: 'tool', label: '学生工具' },
  { value: 'infra', label: '基础设施' },
  { value: 'all', label: '全部' },
] as const

/** PlatformSandboxToolsPage 工具清单:按类别分诊,整行打开详情与修改 */
export default function PlatformSandboxToolsPage() {
  const [query, setQuery] = useUrlState(TOOLS_QUERY)
  const [target, setTarget] = useState<{ tool?: SandboxToolDefinition }>()
  const tools = useAsyncResource(
    () => api.sandbox.listTools(),
    [],
    (list) => list.length === 0
  )
  const all = useMemo(() => tools.data ?? [], [tools.data])

  const counts: Record<string, number> = {
    tool: all.filter((item) => item.category === SANDBOX_COMPONENT_CATEGORY.TOOL).length,
    infra: all.filter((item) => item.category === SANDBOX_COMPONENT_CATEGORY.INFRA).length,
    all: all.length,
  }
  const visible =
    query.category === 'all' ? all : all.filter((item) => item.category === query.category)
  const loaded = tools.status === 'success' || tools.status === 'empty'

  return (
    <PageScaffold>
      <PageHeader
        title="实验工具"
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setTarget({})}>
            登记工具
          </Button>
        }
      />

      <SegmentedControl
        aria-label="按类别筛选"
        options={CATEGORY_SEGMENTS.map((item) => ({
          value: item.value,
          label: item.label,
          count: loaded ? counts[item.value] : undefined,
        }))}
        value={query.category}
        onValueChange={(value) => setQuery({ category: value })}
        className="mb-4 max-w-full overflow-x-auto"
      />

      <ResourceState
        resource={tools}
        emptyIcon={Package}
        emptyTitle="还没有登记实验工具"
        emptyDescription="登记工具后,教师编排实验环境时才能选用。"
        emptyAction={
          <Button variant="primary" leftIcon={Plus} onClick={() => setTarget({})}>
            登记工具
          </Button>
        }
        skeleton={<Skeleton variant="line" lines={6} />}
      >
        {() =>
          visible.length === 0 ? (
            <p className="rounded-lg bg-surface px-4 py-10 text-center text-sm text-ink-sub shadow-xs">
              这个类别下没有工具。
            </p>
          ) : (
            <ul className="divide-y divide-line rounded-lg bg-surface px-2 py-1 shadow-xs">
              {visible.map((tool) => (
                <li key={tool.id}>
                  <ToolRow tool={tool} onOpen={() => setTarget({ tool })} />
                </li>
              ))}
            </ul>
          )
        }
      </ResourceState>

      {target ? (
        <ToolFormModal
          tool={target.tool}
          onClose={() => setTarget(undefined)}
          onSaved={() => {
            setTarget(undefined)
            tools.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

/** ToolRow 一个工具:名称;类型 · 短码 · 生态,不可部署的写原因;右侧状态 */
function ToolRow({ tool, onOpen }: { tool: SandboxToolDefinition; onOpen: () => void }) {
  const reason = tool.resource_spec.disabled_reason
  const deployable = !reason && tool.status === ToolStatus.AVAILABLE
  return (
    <ListRow
      title={tool.name}
      meta={
        reason ? (
          <span className="text-warning">不可部署:{reason}</span>
        ) : (
          <span>
            {sandboxToolKindLabel(tool.kind)} · <span className="font-mono">{tool.code}</span> ·{' '}
            {ecoTagsLabel(tool.eco_tags)}
          </span>
        )
      }
      trailing={
        <StatusIndicator
          tone={reason ? 'warning' : toolStatusTone(tool.status)}
          label={reason ? '不可部署' : deployable ? '可部署' : toolStatusLabel(tool.status)}
        />
      }
      onSelect={onOpen}
    />
  )
}
