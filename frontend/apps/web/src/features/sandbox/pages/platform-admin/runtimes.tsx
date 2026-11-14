// 链运行时(平台侧栏,/platform-admin/runtimes)。设计 §7.6:
// 登记学生做实验用的链环境,走完「填写配置 → 登记镜像 → 自检 → 开放」。十几个。
// 页头 +「登记运行时」;分段「可用 / 接入中 / 已停用 / 全部」(带数字,写进地址);
// 卡片网格,每张卡显示名称与短码、生态、状态、自检结果和接入进度链,没走完的写出下一步;整卡链向运行时详情。
// 登记与修改配置用右侧大抽屉。指标条与常驻说明卡去掉。运行时接口一次返回全部(不分页),计数在本页完成。

import { useMemo, useState } from 'react'
import { Link } from 'react-router'
import { Plus, Server } from 'lucide-react'
import { RuntimeStatus, type SandboxRuntime } from '@chaimir/api-client'
import {
  Button,
  Card,
  ChainProgress,
  PageHeader,
  PageScaffold,
  SegmentedControl,
  Skeleton,
  StatusIndicator,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, useUrlState } from '../../../../hooks'
import { runtimeSelftestStatusLabel, runtimeStatusLabel } from '../../../../utils/labels/sandbox'
import { ONBOARDING_STEPS, runtimeOnboarding } from '../../runtimeOnboarding'
import { runtimeDisabledReason } from '../../runtimeSpec'
import { runtimeSelftestStatusTone, runtimeStatusTone } from '../../statusPresentation'
import { RuntimeFormModal } from './runtime-form'

/** 地址参数:status 分段(默认全部) */
const RUNTIMES_QUERY = {
  status: { defaultValue: 'all', allowed: ['available', 'onboarding', 'disabled', 'all'] },
} as const

const STATUS_SEGMENTS = [
  { value: 'available', label: '可用', status: RuntimeStatus.AVAILABLE },
  { value: 'onboarding', label: '接入中', status: RuntimeStatus.ONBOARDING },
  { value: 'disabled', label: '已停用', status: RuntimeStatus.DISABLED },
  { value: 'all', label: '全部' },
] as const

/** PlatformRuntimesPage 运行时卡片网格:一眼看出每条卡在哪一步 */
export default function PlatformRuntimesPage() {
  const [query, setQuery] = useUrlState(RUNTIMES_QUERY)
  const [formOpen, setFormOpen] = useState(false)
  const runtimes = useAsyncResource(
    () => api.sandbox.listRuntimes(),
    [],
    (list) => list.length === 0
  )
  const all = useMemo(() => runtimes.data ?? [], [runtimes.data])

  const counts = useMemo(() => {
    const result: Record<string, number> = { all: all.length }
    for (const segment of STATUS_SEGMENTS) {
      if ('status' in segment)
        result[segment.value] = all.filter((item) => item.status === segment.status).length
    }
    return result
  }, [all])
  const segment = STATUS_SEGMENTS.find((item) => item.value === query.status)
  const visible =
    segment && 'status' in segment ? all.filter((item) => item.status === segment.status) : all
  const loaded = runtimes.status === 'success' || runtimes.status === 'empty'

  return (
    <PageScaffold>
      <PageHeader
        title="链运行时"
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setFormOpen(true)}>
            登记运行时
          </Button>
        }
      />

      <SegmentedControl
        aria-label="按运行时状态筛选"
        options={STATUS_SEGMENTS.map((item) => ({
          value: item.value,
          label: item.label,
          count: loaded ? counts[item.value] : undefined,
        }))}
        value={query.status}
        onValueChange={(value) => setQuery({ status: value })}
        className="mb-4 max-w-full overflow-x-auto"
      />

      <ResourceState
        resource={runtimes}
        emptyIcon={Server}
        emptyTitle="还没有登记链运行时"
        emptyDescription="登记第一条运行时后,教师才能在实验与竞赛里选择链环境。"
        emptyAction={
          <Button variant="primary" leftIcon={Plus} onClick={() => setFormOpen(true)}>
            登记运行时
          </Button>
        }
        skeleton={<Skeleton variant="block" />}
      >
        {() =>
          visible.length === 0 ? (
            <p className="rounded-lg bg-surface px-4 py-10 text-center text-sm text-ink-sub shadow-xs">
              这个状态下没有运行时。
            </p>
          ) : (
            <ul className="card-grid gap-4">
              {visible.map((runtime) => (
                <li key={runtime.id}>
                  <RuntimeCard runtime={runtime} />
                </li>
              ))}
            </ul>
          )
        }
      </ResourceState>

      {formOpen ? (
        <RuntimeFormModal
          onClose={() => setFormOpen(false)}
          onSaved={() => {
            setFormOpen(false)
            runtimes.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

/** RuntimeCard 一条运行时:名称与短码(整卡链接)、生态、状态、自检结果、接入进度链与下一步 */
function RuntimeCard({ runtime }: { runtime: SandboxRuntime }) {
  const progress = runtimeOnboarding({
    status: runtime.status,
    selftest: runtime.selftest_status,
    availableImages: runtime.available_image_count,
    disabledReason: runtimeDisabledReason(runtime.adapter_spec),
  })
  return (
    <Card interactive className="flex h-full flex-col gap-3 p-5">
      <div className="flex items-start justify-between gap-3">
        <span className="min-w-0">
          <Link
            to={`/platform-admin/runtimes/${runtime.id}`}
            className="card-link line-clamp-1 text-lg font-bold text-ink"
          >
            {runtime.name}
          </Link>
          <span className="block font-mono text-xs text-ink-sub">
            {runtime.code} · {runtime.eco}
          </span>
        </span>
        <StatusIndicator
          tone={runtimeStatusTone(runtime.status)}
          label={runtimeStatusLabel(runtime.status)}
        />
      </div>
      <StatusIndicator
        appearance="text"
        tone={runtimeSelftestStatusTone(runtime.selftest_status)}
        label={runtimeSelftestStatusLabel(runtime.selftest_status)}
      />
      <div className="mt-auto flex flex-col gap-2 pt-1">
        <ChainProgress
          total={ONBOARDING_STEPS.length}
          done={progress.done}
          failedIndexes={progress.failedIndex === undefined ? undefined : [progress.failedIndex]}
          label={`${runtime.name} 接入进度:${ONBOARDING_STEPS.map((step, index) => `${step}${index < progress.done ? '已完成' : ''}`).join('、')}`}
        />
        <p className="text-sm text-ink-sub">{progress.next ?? '已开放给学校,教师可以选用'}</p>
      </div>
    </Card>
  )
}
