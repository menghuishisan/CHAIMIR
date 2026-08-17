// 平台看板页(平台侧栏,/platform-admin/dashboard)。
//
// 看板不是登录首屏(FE-5):平台管理员登录直达学校管理,看板是主动查看的概览。
// 运营统计是看板内页(对齐清单 §3.4),两者同源于 M9 —— 拆成两个侧栏项会让人
// 在两页之间来回对照同一批指标。
//
// 平台看板比学校看板多两项:学校数与待审申请数(后端 Dashboard 的可选字段,
// 只有平台范围才会填),故这两项缺失时不渲染空指标。
// 趋势区与学校看板共用 StatisticsTrendSection(两端接口同形,只有取数不同)。

import { useMemo } from 'react'
import { useNavigate } from 'react-router'
import {
  Activity,
  Book,
  Building,
  FlaskConical,
  Inbox,
  LayoutDashboard,
  Swords,
  Users,
} from 'lucide-react'
import type { Dashboard } from '@chaimir/api-client'
import {
  Breadcrumb,
  Button,
  DescriptionList,
  PageHeader,
  PageScaffold,
  PageSection,
  Skeleton,
  Stat,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import { formatDateTime } from '../../../../utils/formatters'
import { statisticsMetricLabel } from '../../../../utils/labels/admin'
import { StatisticsTrendSection } from '../../StatisticsTrendSection'

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

      <StatisticsTrendSection
        load={(range) => api.admin.getPlatformStatistics(range)}
        idPrefix="platform"
        description="按日聚合的历史快照。先看趋势图,需要逐日数字时在每张图上切到数据表。"
      />
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
      <PageSection title="学校" description="已开通的学校与待处理的入驻申请。">
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
            label="活跃实验环境"
            value={dashboard.active_sandbox_count}
            icon={Activity}
            hint="当前占用运行资源"
          />
        </div>
      </PageSection>

      {quotaItems.length > 0 ? (
        <PageSection title="资源占用快照" description="按平台聚合的实验环境资源用量。">
          <DescriptionList dense columns={2} items={quotaItems} />
        </PageSection>
      ) : null}

      <PageSection>
        <div className="flex flex-wrap items-center justify-between gap-3 well p-4">
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
