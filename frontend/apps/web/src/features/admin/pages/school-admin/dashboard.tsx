// 学校看板页(校管侧栏,/school-admin/dashboard)。
//
// 看板不是登录首屏(FE-5):校管登录直达账号管理,看板是主动查看的概览。
// 一页承载本校当下概览 + 运营统计趋势 —— 统计是看板内页(对齐清单 §3.3),
// 两者同源于 M9,拆成两个侧栏项会让管理员在两页之间来回对照。
//
// 指标带用真实聚合值;趋势区与平台看板共用 StatisticsTrendSection(两端接口同形,只有取数不同)。

import { useNavigate } from 'react-router'
import {
  Activity,
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
 * SchoolAdminDashboardPage 呈现本校概览与运营统计。
 */
export default function SchoolAdminDashboardPage() {
  const dashboard = useAsyncResource(() => api.admin.getSchoolDashboard(), [], () => false)

  return (
    <PageScaffold>
      <PageHeader
        kicker={<Breadcrumb items={[{ label: '概览' }]} />}
        title="学校看板"
        description="本校账号、课程、实验与竞赛的当下概览,以及近期的运营趋势。"
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
        load={(range) => api.admin.getSchoolStatistics(range)}
        idPrefix="school"
        description="按日聚合的历史快照。先看趋势图,需要逐日数字时在每张图上切到数据表。"
      />
    </PageScaffold>
  )
}

/**
 * DashboardContent 渲染概览指标带与资源配额快照。
 *
 * 归族:看板族(规范 §6.5.3 第 ② 族)—— 全族唯一保留 Stat 大卡的一族,
 * 因为这一页的主体就是数字本身。指标带用 `metric-band`(auto-fit),项数变化时不留空版面。
 */
function DashboardContent({ dashboard }: { dashboard: Dashboard }) {
  const navigate = useNavigate()

  return (
    <>
      {/* 教师/学生两项不在这里重复:它们的构成由下方环图回答(§6.5.0 通则 1 不重复说同一件事) */}
      <PageSection title="账号" description="本校账号的开通规模与活跃情况。">
        <div className="metric-band gap-4">
          <Stat label="账号总数" value={dashboard.account_count} icon={Users} hint="含管理员" />
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
        title="资源配额"
        description="实验环境并发与资源上限由平台管理员设定。"
      />

      <DashboardGeneratedFooter generatedAt={dashboard.generated_at}>
        <Button variant="outline" size="sm" onClick={() => navigate('/school-admin/users')}>
          去账号管理
        </Button>
        <Button variant="ghost" size="sm" onClick={() => navigate('/school-admin/system-alerts')}>
          去学校告警
        </Button>
      </DashboardGeneratedFooter>
    </>
  )
}
