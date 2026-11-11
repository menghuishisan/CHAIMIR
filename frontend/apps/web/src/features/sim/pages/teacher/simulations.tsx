// 仿真场景页(教师侧栏,/teacher/simulations)。设计 §7.4:
// 页头 +「提交场景」;默认只看「我提交的」(「全部可用场景」仍可切过去只读浏览);分段按状态(草稿 / 审核中 /
// 已上架 / 已退回),视角、分段与页码写进地址;列表行显示名称、版本、分类、状态;整行打开「审核报告」抽屉;
// 已退回的行直接给「更新后重新提交」,退回原因在审核报告里(列表接口不带审核意见,见设计 §10 缺口)。
//
// 「我提交的」与「可用场景」是同一列表接口的两个视角(GET /sim/packages 按 mine 分叉),作者边界由服务端会话判定。
// 边界:审核通过、驳回、下架、重新上架只在平台管理端;表单也没有「运行方式」—— 教师提交的场景一律
// 在平台的隔离容器内运行(见 docs/04-仿真可视化引擎/02-架构设计.md §8)。

import { useState } from 'react'
import { Network, Pencil, Plus } from 'lucide-react'
import { SIM_PACKAGE_STATUS, type SimPackageMeta, type SimPackageStatus } from '@chaimir/api-client'
import {
  Button,
  Icon,
  ListRow,
  PageHeader,
  PageScaffold,
  Pagination,
  SegmentedControl,
  Skeleton,
  StatusIndicator,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceError } from '../../../../components/ResourceState'
import { usePagedResource, useResourceTotal, useUrlState } from '../../../../hooks'
import { formatDateTime, formatRelativeOccurrence } from '../../../../utils/formatters'
import { simCategoryLabel, simPackageStatusLabel } from '../../../../utils/labels/sim'
import { simPackageStatusTone } from '../../statusPresentation'
import { SimPackageFormModal } from './package-form'
import { SimPackagePreviewModal } from './package-preview'

/** 地址参数:scope 视角,status 状态分段(只对「我提交的」);页码在 page */
const SIM_QUERY = {
  scope: { defaultValue: 'mine', allowed: ['mine', 'available'] },
  status: {
    defaultValue: 'all',
    allowed: ['all', SIM_PACKAGE_STATUS.DRAFT, SIM_PACKAGE_STATUS.REVIEWING, SIM_PACKAGE_STATUS.PUBLISHED, SIM_PACKAGE_STATUS.REJECTED],
  },
} as const

const STATUS_SEGMENTS: { value: 'all' | SimPackageStatus; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: SIM_PACKAGE_STATUS.DRAFT, label: '草稿' },
  { value: SIM_PACKAGE_STATUS.REVIEWING, label: '审核中' },
  { value: SIM_PACKAGE_STATUS.PUBLISHED, label: '已上架' },
  { value: SIM_PACKAGE_STATUS.REJECTED, label: '已退回' },
]

/** 只有草稿与已退回的包可以更新(后端 ensurePackageEditable 同一口径)。 */
function isPackageEditable(status: SimPackageStatus): boolean {
  return status === SIM_PACKAGE_STATUS.DRAFT || status === SIM_PACKAGE_STATUS.REJECTED
}

/**
 * TeacherSimulationsPage 维护教师提交的仿真场景。
 */
export default function TeacherSimulationsPage() {
  const [query, setQuery] = useUrlState(SIM_QUERY)
  const [formTarget, setFormTarget] = useState<{ item?: SimPackageMeta } | undefined>()
  const [previewTarget, setPreviewTarget] = useState<SimPackageMeta>()
  const mine = query.scope === 'mine'

  const packages = usePagedResource<SimPackageMeta>(
    (params) =>
      api.sim.getPackages({
        mine: mine || undefined,
        status: mine ? (query.status === 'all' ? undefined : (query.status as SimPackageStatus)) : SIM_PACKAGE_STATUS.PUBLISHED,
        ...params,
      }),
    [mine, query.status],
    { urlPageKey: 'page' },
  )

  // 分段上的数字取服务端全量口径,固定「我提交的」:它回答「我的场景走到哪一步了」
  const counts = {
    all: useResourceTotal((params) => api.sim.getPackages({ mine: true, ...params }), [], packages.refreshVersion),
    [SIM_PACKAGE_STATUS.DRAFT]: useResourceTotal(
      (params) => api.sim.getPackages({ mine: true, status: SIM_PACKAGE_STATUS.DRAFT, ...params }),
      [],
      packages.refreshVersion,
    ),
    [SIM_PACKAGE_STATUS.REVIEWING]: useResourceTotal(
      (params) => api.sim.getPackages({ mine: true, status: SIM_PACKAGE_STATUS.REVIEWING, ...params }),
      [],
      packages.refreshVersion,
    ),
    [SIM_PACKAGE_STATUS.PUBLISHED]: useResourceTotal(
      (params) => api.sim.getPackages({ mine: true, status: SIM_PACKAGE_STATUS.PUBLISHED, ...params }),
      [],
      packages.refreshVersion,
    ),
    [SIM_PACKAGE_STATUS.REJECTED]: useResourceTotal(
      (params) => api.sim.getPackages({ mine: true, status: SIM_PACKAGE_STATUS.REJECTED, ...params }),
      [],
      packages.refreshVersion,
    ),
  } as Record<string, number | undefined>

  const list = packages.data?.list ?? []
  const filtered = mine && query.status !== 'all'

  return (
    <PageScaffold>
      <PageHeader
        title="仿真场景"
        actions={
          <Button variant="primary" leftIcon={Plus} onClick={() => setFormTarget({})}>
            提交场景
          </Button>
        }
      />

      <div className="mb-5 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        {mine ? (
          <SegmentedControl
            aria-label="按场景状态筛选"
            options={STATUS_SEGMENTS.map((segment) => ({
              value: segment.value,
              label: segment.label,
              count: counts[segment.value],
            }))}
            value={query.status}
            onValueChange={(value) => setQuery({ status: value })}
            className="max-w-full overflow-x-auto"
          />
        ) : (
          <p className="text-sm text-ink-sub">全平台已上架的场景,包含平台内置场景;它们由各自的提交者维护,这里只能查看。</p>
        )}
        <SegmentedControl
          aria-label="切换场景视角"
          size="sm"
          options={[
            { value: 'mine', label: '我提交的' },
            { value: 'available', label: '全部可用场景' },
          ]}
          value={query.scope}
          onValueChange={(value) => setQuery({ scope: value, status: 'all' })}
        />
      </div>

      {packages.status === 'error' ? (
        <ResourceError
          message={packages.error?.message ?? '场景暂时读取不到'}
          traceId={packages.error?.traceId}
          onRetry={packages.reload}
        />
      ) : packages.status === 'empty' ? (
        <div className="flex flex-col items-center rounded-lg bg-surface px-6 py-14 text-center shadow-xs">
          <Icon icon={Network} size="lg" className="text-ink-faint" />
          <p className="mt-3 text-md font-semibold text-ink">
            {mine ? (filtered ? '这个状态下没有场景' : '你还没有提交过场景') : '暂无可用场景'}
          </p>
          <p className="mt-1.5 text-sm text-ink-sub">
            {mine
              ? filtered
                ? '换个状态看看。'
                : '提交场景包后由平台审核,通过即可在课程与实验里使用。'
              : '平台审核通过的场景会显示在这里。'}
          </p>
          {mine && !filtered ? (
            <Button variant="primary" leftIcon={Plus} className="mt-5" onClick={() => setFormTarget({})}>
              提交场景
            </Button>
          ) : null}
        </div>
      ) : packages.status === 'loading' && !packages.data ? (
        <div aria-busy="true" className="rounded-lg bg-surface p-5 shadow-xs">
          <Skeleton variant="line" lines={5} />
        </div>
      ) : (
        <>
          <ul className="divide-y divide-line rounded-lg bg-surface px-2 py-1 shadow-xs" aria-busy={packages.status === 'loading' || undefined}>
            {list.map((pkg) => (
              <li key={pkg.id}>
                <PackageRow
                  pkg={pkg}
                  mine={mine}
                  onOpenReport={() => setPreviewTarget(pkg)}
                  onUpdate={() => setFormTarget({ item: pkg })}
                />
              </li>
            ))}
          </ul>
          <div className="mt-4">
            <Pagination page={packages.page} pageSize={packages.pageSize} total={packages.total} onPageChange={packages.setPage} />
          </div>
        </>
      )}

      {formTarget ? (
        <SimPackageFormModal
          item={formTarget.item}
          onClose={() => setFormTarget(undefined)}
          onSaved={() => {
            setFormTarget(undefined)
            setQuery({ scope: 'mine', status: 'all' })
            packages.reload()
          }}
        />
      ) : null}

      {previewTarget ? <SimPackagePreviewModal item={previewTarget} onClose={() => setPreviewTarget(undefined)} /> : null}
    </PageScaffold>
  )
}

interface PackageRowProps {
  pkg: SimPackageMeta
  /** 「我提交的」视角:整行打开审核报告,可更新的行带更新按钮;可用场景视角只读 */
  mine: boolean
  onOpenReport: () => void
  onUpdate: () => void
}

/**
 * PackageRow 一个场景包:名称、版本与分类、更新时间、状态。
 * 已退回的行写「点开查看退回原因」并给「更新后重新提交」;草稿给「更新」。
 */
function PackageRow({ pkg, mine, onOpenReport, onUpdate }: PackageRowProps) {
  const rejected = pkg.status === SIM_PACKAGE_STATUS.REJECTED
  const meta = (
    <span className="flex flex-wrap items-center gap-x-2">
      <span>
        {simCategoryLabel(pkg.category)} · 版本 {pkg.version}
      </span>
      <span title={formatDateTime(pkg.updated_at)}>· {formatRelativeOccurrence(pkg.updated_at)}更新</span>
      {mine && rejected ? <span className="text-warning">· 点开查看退回原因</span> : null}
    </span>
  )
  const trailing = (
    <span className="flex items-center gap-3">
      <StatusIndicator tone={simPackageStatusTone(pkg.status)} label={simPackageStatusLabel(pkg.status)} />
      {mine && isPackageEditable(pkg.status) ? (
        <Button variant={rejected ? 'secondary' : 'ghost'} size="sm" leftIcon={Pencil} onClick={onUpdate}>
          {rejected ? '更新后重新提交' : '更新'}
        </Button>
      ) : null}
    </span>
  )
  return mine ? (
    <ListRow title={pkg.name} meta={meta} trailing={trailing} onSelect={onOpenReport} />
  ) : (
    <ListRow title={pkg.name} meta={meta} trailing={trailing} />
  )
}
