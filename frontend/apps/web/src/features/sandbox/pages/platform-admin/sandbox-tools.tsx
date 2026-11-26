import { useState } from 'react'
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

const TOOLS_QUERY = {
  category: { defaultValue: 'tool', allowed: ['tool', 'infra', 'all'] },
} as const

const CATEGORY_SEGMENTS = [
  { value: 'tool', label: '学生工具' },
  { value: 'infra', label: '基础设施' },
  { value: 'all', label: '全部' },
] as const

export default function PlatformSandboxToolsPage() {
  const [query, setQuery] = useUrlState(TOOLS_QUERY)
  const [target, setTarget] = useState<{ tool?: SandboxToolDefinition }>()
  const tools = useAsyncResource(
    () => api.sandbox.listTools(),
    [],
    (list) => list.length === 0
  )
  const all = tools.data ?? []

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
