// 平台看板页(平台侧栏,/platform-admin/dashboard)。
//
// 看板不是登录首屏(FE-5):平台管理员登录直达学校管理,看板是主动查看的概览。
// 运营统计是看板内页(对齐清单 §3.4),两者同源于 M9 —— 拆成两个侧栏项会让人
// 在两页之间来回对照同一批指标。
//
// 平台看板比学校看板多两项:学校数与待审申请数(后端 Dashboard 的可选字段,
// 只有平台范围才会填),故这两项缺失时不渲染空指标。
// 趋势区与学校看板共用 StatisticsTrendSection(两端接口同形,只有取数不同)。

import { useNavigate } from 'react-router'
import {
  Activity,
  Building,
  Inbox,
  LayoutDashboard,
  Users,
} from 'lucide-react'
import type { Dashboard } from '@chaimir/api-client'
import {
  Breadcrumb,
  Button,
  PageHeader,
  PageScaffold,
  PageSection,
  Skeleton,
  Stat,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource } from '../../../../hooks'
import {
  DashboardCompositionSection,
  DashboardGeneratedFooter,
  DashboardQuotaSection,
  DashboardTeachingPracticeSection,
} from '../../components/DashboardSections'
import { StatisticsTrendSection } from '../../components/StatisticsTrendSection'

/**
 * PlatformDashboardPage 呈现全平台概览与运营统计。
 */
export default function PlatformDashboardPage() {
  const dashboard = useAsyncResource(() => api.admin.getPlatformDashboard(), [], () => false)

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '运营' }]} />}
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
 *
 * 归族:看板族(规范 §6.5.3 第 ② 族)—— 全族唯一保留 Stat 大卡的一族,
 * 因为这一页的主体就是数字本身。指标带用 `metric-band`(auto-fit)而不是定列数栅格:
 * 「师生构成」只有两项,摊进四列栅格会拉成两张半屏宽的巨卡。
 */
function DashboardContent({ dashboard }: { dashboard: Dashboard }) {
  const navigate = useNavigate()

  return (
    <>
      <PageSection title="学校" description="已开通的学校与待处理的入驻申请。">
        <div className="metric-band gap-4">
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

      <DashboardCompositionSection dashboard={dashboard} />

      <DashboardTeachingPracticeSection dashboard={dashboard} />

      <DashboardQuotaSection
        snapshot={dashboard.resource_quota_snapshot}
        title="资源占用快照"
        description="按平台聚合的实验环境资源上限。"
      />

      <DashboardGeneratedFooter generatedAt={dashboard.generated_at}>
        <Button variant="outline" size="sm" onClick={() => navigate('/platform-admin/schools')}>
          去学校管理
        </Button>
        <Button variant="ghost" size="sm" onClick={() => navigate('/platform-admin/alerts')}>
          去告警中心
        </Button>
        <Button variant="ghost" size="sm" onClick={() => navigate('/platform-admin/monitoring')}>
          去监控面板
        </Button>
      </DashboardGeneratedFooter>
    </>
  )
}
