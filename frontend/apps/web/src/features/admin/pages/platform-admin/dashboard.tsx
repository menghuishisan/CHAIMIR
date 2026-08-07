// 平台看板页(平台侧栏,/platform-admin/dashboard)。
//
// 看板不是登录首屏(FE-5):平台管理员登录直达学校管理,看板是主动查看的概览。
// 运营统计是看板内页(对齐清单 §3.4),两者同源于 M9 —— 拆成两个侧栏项会让人
// 在两页之间来回对照同一批指标。
//
// 平台看板比学校看板多两项:学校数与待审申请数(后端 Dashboard 的可选字段,
// 只有平台范围才会填),故这两项缺失时不渲染空指标。

import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import {
  Activity,
  Book,
  Building,
  FlaskConical,
  Inbox,
  LayoutDashboard,
  Swords,
  TrendingUp,
  Users,
} from 'lucide-react'
import type { Dashboard, Statistics } from '@chaimir/api-client'
import {
  Badge,
  Breadcrumb,
  Button,
  Callout,
  DescriptionList,
  Empty,
  FormField,
  Input,
  PageHeader,
  PageScaffold,
  PageSection,
  Skeleton,
  Stat,
  Table,
  type TableColumn,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDate, formatDateTime } from '../../../../utils/formatters'
import { statisticsMetricLabel } from '../../../../utils/labels/admin'

/** 趋势默认区间:近 30 天。 */
const DEFAULT_RANGE_DAYS = 30

/**
 * PlatformDashboardPage 呈现全平台概览与运营统计。
 */
export default function PlatformDashboardPage() {
  const dashboard = useAsyncResource(() => api.admin.getPlatformDashboard(), [], () => false)

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '运营' }, { label: '平台看板' }]} />}
        title="平台看板"
        description="全平台学校、账号、课程、实验与竞赛的当下概览,以及近期的运营趋势。"
        icon={LayoutDashboard}
      />

      <ResourceState
        resource={dashboard}
        emptyIcon={LayoutDashboard}
        emptyTitle="暂无看板数据"
        emptyDescription="学校开始使用后会在这里显示概览。"
        skeleton={
          <div className="flex flex-col gap-4">
            <Skeleton variant="block" />
            <Skeleton variant="line" lines={4} />
          </div>
        }
      >
        {(data) => <DashboardContent dashboard={data} />}
      </ResourceState>

      <StatisticsSection />
    </PageScaffold>
  )
}

/**
 * DashboardContent 渲染平台概览指标带与资源配额快照。
 */
function DashboardContent({ dashboard }: { dashboard: Dashboard }) {
  const navigate = useNavigate()

  const quotaItems = useMemo(() => {
    const snapshot = dashboard.resource_quota_snapshot
    if (!snapshot) return []
    return Object.entries(snapshot).map(([key, value]) => ({
      term: statisticsMetricLabel(key) ?? quotaTermLabel(key),
      description: typeof value === 'number' || typeof value === 'string' ? String(value) : '—',
      mono: true,
    }))
  }, [dashboard.resource_quota_snapshot])

  return (
    <>
      <PageSection title="租户" description="已开通的学校与待处理的入驻申请。">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat label="学校数" value={dashboard.tenant_count ?? 0} icon={Building} />
          <Stat
            label="待审入驻申请"
            value={dashboard.pending_apply_count ?? 0}
            icon={Inbox}
            hint={(dashboard.pending_apply_count ?? 0) > 0 ? '需要你处理' : '暂时没有积压'}
          />
          <Stat label="账号总数" value={dashboard.account_count} icon={Users} />
          <Stat
            label="活跃账号"
            value={dashboard.active_account_count}
            icon={Activity}
            hint="近期有登录记录"
          />
        </div>
      </PageSection>

      <PageSection title="师生构成" description="全平台教师与学生的开通规模。">
        <div className="grid gap-4 sm:grid-cols-2">
          <Stat label="教师" value={dashboard.teacher_count} icon={Users} />
          <Stat label="学生" value={dashboard.student_count} icon={Users} />
        </div>
      </PageSection>

      <PageSection title="教学与实践" description="课程、实验与竞赛的开展情况。">
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Stat
            label="课程"
            value={dashboard.course_count}
            icon={Book}
            hint={`进行中 ${dashboard.active_course_count}`}
          />
          <Stat
            label="实验"
            value={dashboard.experiment_count}
            icon={FlaskConical}
            hint={`活跃环境 ${dashboard.active_instance_count}`}
          />
          <Stat
            label="竞赛"
            value={dashboard.contest_count}
            icon={Swords}
            hint={`进行中 ${dashboard.active_contest_count}`}
          />
          <Stat
            label="活跃沙箱"
            value={dashboard.active_sandbox_count}
            icon={Activity}
            hint="当前占用运行资源"
          />
        </div>
      </PageSection>

      {quotaItems.length > 0 ? (
        <PageSection title="资源占用快照" description="按平台聚合的沙箱资源用量。">
          <DescriptionList dense columns={2} items={quotaItems} />
        </PageSection>
      ) : null}

      <PageSection>
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-line bg-surface-sunken p-4">
          <div className="min-w-0">
            <p className="text-sm text-ink">数据生成于 {formatDateTime(dashboard.generated_at)}</p>
            <p className="text-xs text-ink-sub">
              概览按周期聚合,不是实时值。需要看实时状态请去对应功能页。
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button variant="outline" size="sm" onClick={() => navigate('/platform-admin/schools')}>
              去学校管理
            </Button>
            <Button variant="ghost" size="sm" onClick={() => navigate('/platform-admin/alerts')}>
              去告警中心
            </Button>
            <Button variant="ghost" size="sm" onClick={() => navigate('/platform-admin/monitoring')}>
              去监控面板
            </Button>
          </div>
        </div>
      </PageSection>
    </>
  )
}

/** MetricRow 是统计表格的一行:一个日期 + 一个指标 + 值。 */
interface MetricRow {
  date: string
  term: string
  value: string
}

/**
 * StatisticsSection 展示按日聚合的平台运营统计。
 * metrics 是开放对象:页面把它摊平成「日期 × 指标」行,未登记的指标键跳过,
 * 不猜语义也不把内部键名抛到界面上。
 */
function StatisticsSection() {
  const [from, setFrom] = useState(() => isoDate(-DEFAULT_RANGE_DAYS))
  const [to, setTo] = useState(() => isoDate(0))
  const [range, setRange] = useState({ from: isoDate(-DEFAULT_RANGE_DAYS), to: isoDate(0) })

  const statistics = useAsyncResource(
    () => api.admin.getPlatformStatistics({ from: range.from, to: range.to }),
    [range.from, range.to],
    (value) => value.length === 0,
  )

  const rows = useMemo<MetricRow[]>(() => {
    const list = statistics.data ?? []
    const out: MetricRow[] = []
    for (const item of list) {
      for (const [key, value] of Object.entries(item.metrics)) {
        const term = statisticsMetricLabel(key)
        if (!term) continue
        out.push({
          date: item.date,
          term,
          value: typeof value === 'number' ? String(value) : typeof value === 'string' ? value : '—',
        })
      }
    }
    return out
  }, [statistics.data])

  const latest = useMemo<Statistics | undefined>(() => {
    const list = statistics.data ?? []
    return list.length > 0 ? list[list.length - 1] : undefined
  }, [statistics.data])

  const columns: TableColumn<MetricRow>[] = [
    {
      key: 'date',
      header: '日期',
      render: (row) => (
        <span className="whitespace-nowrap font-mono text-xs tabular-nums text-ink-sub">
          {formatDate(row.date)}
        </span>
      ),
    },
    { key: 'term', header: '指标' },
    { key: 'value', header: '数值', align: 'right', mono: true },
  ]

  return (
    <PageSection
      title="运营统计"
      description="按日聚合的历史快照。表格是无障碍的等价呈现,不做无来源的装饰图形。"
      actions={
        <form
          className="flex flex-wrap items-end gap-2"
          onSubmit={(event) => {
            event.preventDefault()
            setRange({ from, to })
          }}
        >
          <FormField label="开始日期" htmlFor="platform-stats-from" className="mb-0">
            <Input
              id="platform-stats-from"
              type="date"
              value={from}
              onChange={(event) => setFrom(event.target.value)}
            />
          </FormField>
          <FormField label="结束日期" htmlFor="platform-stats-to" className="mb-0">
            <Input
              id="platform-stats-to"
              type="date"
              value={to}
              onChange={(event) => setTo(event.target.value)}
            />
          </FormField>
          <Button type="submit" variant="outline" size="sm" leftIcon={TrendingUp}>
            查看
          </Button>
        </form>
      }
    >
      <div className="flex flex-col gap-4">
        {latest ? (
          <div className="flex flex-wrap items-center gap-2">
            <Badge tone="neutral">最新快照 {formatDate(latest.date)}</Badge>
            <Badge tone="jade">共 {(statistics.data ?? []).length} 天数据</Badge>
          </div>
        ) : null}

        <ResourceState
          resource={statistics}
          emptyIcon={TrendingUp}
          emptyTitle="暂无统计数据"
          emptyDescription="这个区间内还没有聚合快照。统计每日生成,新部署要等一天。"
          skeleton={<Table columns={columns} data={[]} rowKey={() => ''} loading />}
        >
          {() =>
            rows.length === 0 ? (
              <Empty
                icon={TrendingUp}
                title="这段时间没有可呈现的指标"
                description="快照里的指标都还未登记名称,已跳过以避免显示内部键名。"
              />
            ) : (
              <Table columns={columns} data={rows} rowKey={(row) => `${row.date}-${row.term}`} />
            )
          }
        </ResourceState>

        <Callout tone="info">
          需要更细的实时数据请去监控面板;单所学校的用量在学校详情里看配额与活跃沙箱。
        </Callout>
      </div>
    </PageSection>
  )
}

/** isoDate 按天偏移生成 date 控件需要的 YYYY-MM-DD。 */
function isoDate(offsetDays: number): string {
  const date = new Date()
  date.setDate(date.getDate() + offsetDays)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

/**
 * quotaTermLabel 给资源快照里未登记的键一个通用名。
 * 快照键由后端聚合结构决定,平台端只读;未登记键给通用名而不是裸键名。
 */
function quotaTermLabel(key: string): string {
  const known: Record<string, string> = {
    max_concurrent: '并发上限',
    max_cpu: 'CPU 上限',
    max_memory_mb: '内存上限(MB)',
    used_cpu: '已用 CPU',
    used_memory_mb: '已用内存(MB)',
  }
  return known[key] ?? '其他资源指标'
}
