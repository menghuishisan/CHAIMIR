// 仿真场景页(教师侧栏,/teacher/simulations)。
//
// 教师在这里维护自己提交的仿真包:提交新包、更新被退回或仍是草稿的包、查看审核前预览报告。
// 「我提交的」与「可用场景」是同一列表接口的两个视角(GET /sim/packages 按 mine 分叉),
// 作者边界由服务端会话判定,页面不传作者参数。
//
// 边界(对齐清单 §3.2 / §6.1):审核通过、驳回、下架、重新上架只在平台管理端,本页不出现这些动作。
// 表单也没有「运行方式」:教师提交的场景一律在平台的隔离容器内运行,执行位置按代码来源派生
// (见 docs/04-仿真可视化引擎/02-架构设计.md §8),不是提交时的可选项。

import { useState } from 'react'
import { CircleCheck, FileSearch, Network, Pencil, Plus, Upload } from 'lucide-react'
import {
  SIM_COMPUTE,
  SIM_PACKAGE_STATUS,
  type SimPackageMeta,
  type SimPackageStatus,
} from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  FilterBar,
  FilterField,
  PageHeader,
  PageScaffold,
  PageSection,
  Pagination,
  SegmentedControl,
  Stat,
  StatusIndicator,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { usePagedResource, useResourceTotal } from '../../../../hooks'
import { formatShortDateTime } from '../../../../utils/formatters'
import {
  simCategoryLabel,
  simComputeLabel,
  simPackageStatusLabel,
  simPackageStatusTone,
} from '../../../../utils/labels/sim'
import { SimPackageFormModal } from './package-form'
import { SimPackagePreviewModal } from './package-preview'

/** 状态筛选项:值为空串表示不过滤(只在「我提交的」视角可用)。 */
const STATUS_FILTERS = [
  { value: '', label: '全部' },
  { value: SIM_PACKAGE_STATUS.DRAFT, label: '草稿' },
  { value: SIM_PACKAGE_STATUS.REVIEWING, label: '审核中' },
  { value: SIM_PACKAGE_STATUS.PUBLISHED, label: '已上架' },
  { value: SIM_PACKAGE_STATUS.REJECTED, label: '已退回' },
] as const

/** 视角切换:我提交的(可维护)与全部可用场景(只读浏览)。 */
const SCOPE_FILTERS = [
  { value: 'mine', label: '我提交的' },
  { value: 'available', label: '可用场景' },
] as const

/** 只有草稿与已退回的包可以更新(后端 ensurePackageEditable 同一口径)。 */
function isPackageEditable(status: SimPackageStatus): boolean {
  return status === SIM_PACKAGE_STATUS.DRAFT || status === SIM_PACKAGE_STATUS.REJECTED
}

/**
 * TeacherSimulationsPage 维护教师提交的仿真场景。
 */
export default function TeacherSimulationsPage() {
  const [scope, setScope] = useState<string>('mine')
  const [statusFilter, setStatusFilter] = useState<string>('')
  const [formTarget, setFormTarget] = useState<{ item?: SimPackageMeta } | undefined>()
  const [previewTarget, setPreviewTarget] = useState<SimPackageMeta>()

  const mine = scope === 'mine'

  const packages = usePagedResource<SimPackageMeta>(
    (params) =>
      api.sim.getPackages({
        mine: mine || undefined,
        status: mine
          ? statusFilter
            ? (statusFilter as SimPackageStatus)
            : undefined
          : SIM_PACKAGE_STATUS.PUBLISHED,
        ...params,
      }),
    [mine, statusFilter],
  )

  // 指标带取服务端全量口径,且固定「我提交的」口径,不随下方状态筛选变化:
  // 它回答「我提交的场景走到哪一步了」。全部可用场景那一档没有可分桶的服务端聚合,
  // 数量由下方分组说明的 total 承载(规范 §6.5:拿不到全量聚合就不做这张卡)。
  const mineTotalCount = useResourceTotal(
    (params) => api.sim.getPackages({ mine: true, ...params }),
    [],
  )
  const minePublishedCount = useResourceTotal(
    (params) =>
      api.sim.getPackages({ mine: true, status: SIM_PACKAGE_STATUS.PUBLISHED, ...params }),
    [],
  )
  const mineReviewingCount = useResourceTotal(
    (params) =>
      api.sim.getPackages({ mine: true, status: SIM_PACKAGE_STATUS.REVIEWING, ...params }),
    [],
  )
  const mineRejectedCount = useResourceTotal(
    (params) =>
      api.sim.getPackages({ mine: true, status: SIM_PACKAGE_STATUS.REJECTED, ...params }),
    [],
  )

  const columns: TableColumn<SimPackageMeta>[] = [
    {
      key: 'name',
      header: '仿真场景',
      render: (pkg) => (
        <div className="min-w-0">
          <div className="truncate font-medium text-ink">{pkg.name}</div>
          <div className="truncate text-xs text-ink-sub">
            {simCategoryLabel(pkg.category)} · 版本 {pkg.version}
          </div>
        </div>
      ),
    },
    {
      key: 'compute',
      header: '运行位置',
      render: (pkg) => (
        <Badge tone={pkg.compute === SIM_COMPUTE.BROWSER ? 'jade' : 'info'}>
          {simComputeLabel(pkg.compute)}
        </Badge>
      ),
    },
    {
      key: 'updated_at',
      header: '更新时间',
      render: (pkg) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatShortDateTime(pkg.updated_at)}
        </span>
      ),
    },
    {
      key: 'status',
      header: '状态',
      render: (pkg) => (
        <StatusIndicator tone={simPackageStatusTone(pkg.status)} label={simPackageStatusLabel(pkg.status)} />
      ),
    },
    {
      key: 'actions',
      header: '操作',
      align: 'right',
      render: (pkg) =>
        mine ? (
          <div className="flex items-center justify-end gap-1">
            <Button variant="ghost" size="sm" leftIcon={FileSearch} onClick={() => setPreviewTarget(pkg)}>
              审核报告
            </Button>
            {isPackageEditable(pkg.status) ? (
              <Button variant="ghost" size="sm" leftIcon={Pencil} onClick={() => setFormTarget({ item: pkg })}>
                更新
              </Button>
            ) : null}
          </div>
        ) : (
          <span className="text-sm text-ink-faint">仅浏览</span>
        ),
    },
  ]

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '资源' }, { label: '仿真场景' }]} />}
        title="仿真场景"
        description="提交与维护你的仿真场景包。提交后由平台审核,通过后学生即可在仿真实验室里使用。"
        icon={Network}
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
            提交场景
          </Button>
        }
      />

      <PageSection>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="我提交的场景" value={mineTotalCount ?? '—'} icon={Network} hint="不受下方筛选影响" />
          <Stat label="已上架" value={minePublishedCount ?? '—'} icon={CircleCheck} hint="学生可使用" />
          <Stat label="审核中" value={mineReviewingCount ?? '—'} icon={Upload} hint="等待平台审核" />
          <Stat
            label="已退回"
            value={mineRejectedCount ?? '—'}
            icon={Pencil}
            hint={mineRejectedCount === 0 ? '暂无退回' : '按审核意见修改后重新提交'}
          />
        </div>
      </PageSection>

      <PageSection
        title={mine ? '我提交的场景' : '全部可用场景'}
        description={
          mine
            ? `共 ${packages.total} 个场景。草稿与已退回的可以更新后重新提交。`
            : `共 ${packages.total} 个已上架场景。这些场景可以在实验编排里作为组件引用。`
        }
      >
        <div className="flex flex-col gap-4">
          <FilterBar label="场景包筛选">
            <FilterField label="场景视角" group>
              <SegmentedControl
                aria-label="切换场景视角"
                size="sm"
                options={SCOPE_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                value={scope}
                onValueChange={(value) => {
                  setScope(value)
                  setStatusFilter('')
                }}
              />
            </FilterField>
            {mine ? (
              <FilterField label="场景状态" group>
                <SegmentedControl
                  aria-label="按场景状态筛选"
                  size="sm"
                  options={STATUS_FILTERS.map((item) => ({ value: item.value, label: item.label }))}
                  value={statusFilter}
                  onValueChange={setStatusFilter}
                />
              </FilterField>
            ) : null}
          </FilterBar>
          {mine ? null : (
            <Callout tone="info">
              这里是全平台已上架的场景,包含平台内置场景。它们由各自的提交者维护,你只能查看。
            </Callout>
          )}

          <ResourceState
            resource={packages}
            emptyIcon={Network}
            emptyTitle={mine ? (statusFilter ? '这个状态下没有场景' : '你还没有提交过场景') : '暂无可用场景'}
            emptyDescription={
              mine
                ? statusFilter
                  ? '换个状态看看。'
                  : '提交场景包后由平台审核,通过即可在课程与实验里使用。'
                : '平台审核通过的场景会显示在这里。'
            }
            emptyAction={
              mine && !statusFilter ? (
                <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
                  提交场景
                </Button>
              ) : undefined
            }
            skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
          >
            {(page) => (
              <>
                <Table columns={columns} data={page.list} rowKey={(item) => item.id} />
                <Pagination
                  page={packages.page}
                  pageSize={packages.pageSize}
                  total={packages.total}
                  onPageChange={packages.setPage}
                />
              </>
            )}
          </ResourceState>
        </div>
      </PageSection>

      {formTarget ? (
        <SimPackageFormModal
          item={formTarget.item}
          onClose={() => setFormTarget(undefined)}
          onSaved={() => {
            setFormTarget(undefined)
            setScope('mine')
            packages.reload()
          }}
        />
      ) : null}

      {previewTarget ? (
        <SimPackagePreviewModal item={previewTarget} onClose={() => setPreviewTarget(undefined)} />
      ) : null}
    </PageScaffold>
  )
}
