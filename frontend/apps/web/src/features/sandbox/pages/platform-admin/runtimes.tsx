// 链运行时页(平台侧栏,/platform-admin/runtimes)。
//
// 一条链要能给学生用,得走完四步:登记声明 → 登记镜像 → 预拉取到各节点 → 自检通过。
// 后端把这条链路做成硬前置(RunRuntimeSelftest 要求默认镜像已预拉取成功且内置创世,
// 自检通过后运行时才转为可用),所以本页把它当作一条进度呈现,而不是一堆互不相关的按钮 ——
// 管理员看一眼就知道卡在第几步。
//
// 镜像、自检与预拉取的具体操作在运行时详情深页:那是按单条运行时展开的多轮操作,
// 塞进列表会让「有哪些运行时、各自到哪一步」这件常做的事变慢。

import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { CircleCheck, Plus, Server, Settings2, ShieldCheck } from 'lucide-react'
import { RuntimeSelftestStatus, RuntimeStatus, type SandboxRuntime } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  Card,
  CardBody,
  CardHeader,
  ChainProgress,
  DescriptionList,
  FilterBar,
  FilterField,
  PageHeader,
  PageScaffold,
  PageSection,
  SegmentedControl,
  Skeleton,
  Stat,
  StatusIndicator,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import {
  runtimeSelftestStatusLabel,
  runtimeStatusLabel,
} from '../../../../utils/labels/sandbox'
import { runtimeSelftestStatusTone, runtimeStatusTone } from '../../statusPresentation'
import { RuntimeFormModal } from './runtime-form'
import { runtimeHasDeployCommand, runtimeWorkspaceDir } from '../../runtimeSpec'

/** 接入四步:与后端的硬前置一致(镜像 → 预拉取 → 自检 → 可用)。 */
const ONBOARDING_STEPS = ['填写环境配置', '登记镜像并预拉取', '自检通过', '开放给学校'] as const

/** 适配层级说明:列表上只给一句话,详细含义在登记表单里。 */
const ADAPTER_LEVEL_LABELS: Record<number, string> = {
  1: '只托管环境',
  2: '声明标准链能力',
  3: '自带插件实现',
}

/** 状态筛选项:值为空串表示不过滤。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: String(RuntimeStatus.AVAILABLE), label: '可用' },
  { value: String(RuntimeStatus.ONBOARDING), label: '接入中' },
  { value: String(RuntimeStatus.DISABLED), label: '已停用' },
] as const

/**
 * PlatformRuntimesPage 列出链运行时并承载登记与修改。
 */
export default function PlatformRuntimesPage() {
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [formTarget, setFormTarget] = useState<{ runtime?: SandboxRuntime } | undefined>()

  const runtimes = useAsyncResource(() => api.sandbox.listRuntimes(), [], (value) => value.length === 0)

  const list = useMemo(() => runtimes.data ?? [], [runtimes.data])

  const visible = useMemo(
    () => (statusFilter ? list.filter((item) => item.status === Number(statusFilter)) : list),
    [list, statusFilter],
  )

  const stats = useMemo(
    () => ({
      available: list.filter((item) => item.status === RuntimeStatus.AVAILABLE).length,
      onboarding: list.filter((item) => item.status === RuntimeStatus.ONBOARDING).length,
      failed: list.filter((item) => item.selftest_status === RuntimeSelftestStatus.FAILED).length,
    }),
    [list],
  )

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '底层资源' }, { label: '链运行时' }]} />}
        title="链运行时"
        description="学生做实验和答题时使用的链环境。填写配置后还要登记镜像、预拉取并自检通过,才会开放给学校。"
        icon={Server}
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
            登记运行时
          </Button>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="运行时总数" value={list.length} icon={Server} />
          <Stat label="可用" value={stats.available} icon={CircleCheck} hint="学校可以选用" />
          <Stat label="接入中" value={stats.onboarding} icon={Settings2} hint="还没通过自检" />
          <Stat
            label="自检未通过"
            value={stats.failed}
            icon={ShieldCheck}
            hint={stats.failed > 0 ? '需要查看失败原因' : '暂无失败记录'}
          />
        </div>
      </PageSection>

      <PageSection
        title="运行时列表"
        description="按状态筛选。点进详情做镜像登记、预拉取与自检。"
      >
        <div className="flex flex-col gap-4">
          <FilterBar label="运行时筛选">
            <FilterField label="运行时状态" group>
              <SegmentedControl
                aria-label="按运行时状态筛选"
                size="sm"
                options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={statusFilter}
                onValueChange={setStatusFilter}
              />
            </FilterField>
          </FilterBar>

          <ResourceState
            resource={runtimes}
            emptyIcon={Server}
            emptyTitle="还没有登记链运行时"
            emptyDescription="登记第一条运行时后,教师才能在实验与竞赛里选择链环境。"
            emptyAction={
              <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
                登记运行时
              </Button>
            }
            skeleton={<Skeleton variant="line" lines={4} />}
          >
            {() =>
              visible.length === 0 ? (
                <Callout tone="info">这个状态下没有运行时,换个状态看看。</Callout>
              ) : (
                <div className="grid gap-4 lg:grid-cols-2">
                  {visible.map((runtime) => (
                    <RuntimeCard
                      key={runtime.id}
                      runtime={runtime}
                      onEdit={() => setFormTarget({ runtime })}
                    />
                  ))}
                </div>
              )
            }
          </ResourceState>

          <Callout tone="info">
            停用运行时不影响已创建的环境,只是之后不再分配。要换镜像版本请在详情里登记新版本并设为默认。
          </Callout>
        </div>
      </PageSection>

      {formTarget ? (
        <RuntimeFormModal
          runtime={formTarget.runtime}
          onClose={() => setFormTarget(undefined)}
          onSaved={() => {
            setFormTarget(undefined)
            runtimes.reload()
          }}
        />
      ) : null}
    </PageScaffold>
  )
}

interface RuntimeCardProps {
  runtime: SandboxRuntime
  onEdit: () => void
}

/**
 * RuntimeCard 展示单条运行时的接入进度与入口。
 */
function RuntimeCard({ runtime, onEdit }: RuntimeCardProps) {
  const navigate = useNavigate()

  // 接入进度:登记必然已完成;镜像与预拉取的细节在详情页,列表用自检结果反推 ——
  // 自检的硬前置就是默认镜像预拉取成功,所以自检通过即意味着前两步都过了。
  const done = useMemo(() => {
    if (runtime.status === RuntimeStatus.AVAILABLE) return ONBOARDING_STEPS.length
    if (runtime.selftest_status === RuntimeSelftestStatus.PASSED) return 3
    return 1
  }, [runtime.selftest_status, runtime.status])

  const failedIndexes = runtime.selftest_status === RuntimeSelftestStatus.FAILED ? [2] : []

  const openDetail = useCallback(
    () => navigate(`/platform-admin/runtimes/${runtime.id}`),
    [navigate, runtime.id],
  )

  return (
    <Card>
      <CardHeader
        title={runtime.name}
        description={`${runtime.eco} · ${ADAPTER_LEVEL_LABELS[runtime.adapter_level] ?? '未登记的适配层级'}`}
        actions={
          <StatusIndicator
            tone={runtimeStatusTone(runtime.status)}
            label={runtimeStatusLabel(runtime.status)}
          />
        }
      />
      <CardBody className="flex flex-col gap-4">
        <DescriptionList
          dense
          items={[
            { term: '运行时短名', description: runtime.code, mono: true },
            {
              term: '工作区目录',
              description: runtimeWorkspaceDir(runtime.adapter_spec),
              mono: true,
            },
            {
              term: '链能力来源',
              description: capabilitySourceLabel(runtime),
            },
          ]}
        />

        <div className="flex flex-col gap-2">
          <div className="flex flex-wrap items-center gap-2">
            <StatusIndicator
              tone={runtimeSelftestStatusTone(runtime.selftest_status)}
              label={runtimeSelftestStatusLabel(runtime.selftest_status)}
            />
            <Badge tone="neutral">
              {ONBOARDING_STEPS[Math.min(done, ONBOARDING_STEPS.length - 1)]}
            </Badge>
          </div>
          <ChainProgress
            total={ONBOARDING_STEPS.length}
            done={done}
            failedIndexes={failedIndexes}
            label={`${runtime.name} 接入进度`}
          />
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button variant="outline" size="sm" onClick={openDetail}>
            镜像与自检
          </Button>
          <Button variant="ghost" size="sm" leftIcon={Settings2} onClick={onEdit}>
              修改配置
          </Button>
        </div>
      </CardBody>
    </Card>
  )
}

/**
 * capabilitySourceLabel 说明这条运行时的链能力从哪来。
 * 三种来源(内置实现标识 / 插件 / 清单里的能力命令)由后端 validateRuntimeRequest 三选一要求,
 * 界面直接说明结论,不让管理员自己拼。
 */
function capabilitySourceLabel(runtime: SandboxRuntime): string {
  if (runtime.plugin_ref.trim() !== '') return '运行时插件'
  if (runtime.capability_impl.trim() !== '') return '平台内置实现'
  if (runtimeHasDeployCommand(runtime.adapter_spec)) return '清单声明的链能力命令'
  return '仅托管环境,链操作由学生自行完成'
}
