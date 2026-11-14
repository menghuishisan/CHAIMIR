// 学校看板页(校管侧栏,/school-admin/dashboard)。设计 §7.5:
// 第一行关键数字小卡(自动排列):账号(总数 / 活跃)、师生(教师 / 学生,卡内比例条)、课程(进行中 / 总数)、
// 实验(数量 / 运行中实例)、竞赛(进行中 / 总数)、实验环境(运行中 · 配额使用比例条);
// 第二行「需要处理」(待审核成绩、待处理申诉、待处理告警,数字用已有列表的总数接口取,每项可点进对应页面);
// 下方趋势图按数据形状选图,时间范围切换在图表区右上角。
// 看板不是登录首屏(FE-5):校管登录直达账号管理,看板是主动查看的概览。

import { LayoutDashboard } from 'lucide-react'
import { AlertStatus, GradeAppealStatus, GradeReviewStatus } from '@chaimir/api-client'
import { PageHeader, PageScaffold, Skeleton } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, useResourceTotal } from '../../../../hooks'
import { DashboardAttention, DashboardGeneratedNote, DashboardKeyFigures } from '../../components/DashboardSections'
import { StatisticsTrendSection } from '../../components/StatisticsTrendSection'

/**
 * SchoolAdminDashboardPage 呈现本校概览、需要处理的事与运营趋势。
 */
export default function SchoolAdminDashboardPage() {
  const dashboard = useAsyncResource(() => api.admin.getSchoolDashboard(), [], () => false)
  // 「需要处理」的数字取各列表的服务端总数(只取一条),与对应页面同一口径
  const pendingReviews = useResourceTotal(
    (params) => api.grade.listReviews({ status: GradeReviewStatus.PENDING, ...params }),
    [],
    dashboard.refreshVersion,
  )
  const pendingAppeals = useResourceTotal(
    (params) => api.grade.listAppeals({ status: GradeAppealStatus.PENDING, ...params }),
    [],
    dashboard.refreshVersion,
  )
  const pendingAlerts = useResourceTotal(
    (params) => api.admin.listAlertEvents({ status: AlertStatus.PENDING, ...params }),
    [],
    dashboard.refreshVersion,
  )

  return (
    <PageScaffold>
      <PageHeader
        title="学校看板"
        actions={dashboard.status === 'success' && dashboard.data ? <DashboardGeneratedNote generatedAt={dashboard.data.generated_at} /> : undefined}
      />

      <ResourceState
        resource={dashboard}
        emptyIcon={LayoutDashboard}
        emptyTitle="暂无看板数据"
        emptyDescription="学校开始使用后会在这里显示概览。"
        skeleton={
          <div className="metric-band mb-6 gap-4" aria-busy="true">
            {Array.from({ length: 6 }, (_, index) => (
              <div key={index} className="rounded-lg bg-surface p-4 shadow-xs">
                <Skeleton variant="line" lines={3} />
              </div>
            ))}
          </div>
        }
      >
        {(data) => <DashboardKeyFigures dashboard={data} />}
      </ResourceState>

      <DashboardAttention
        items={[
          { key: 'reviews', label: '待审核成绩', count: pendingReviews, to: '/school-admin/approvals?status=pending' },
          { key: 'appeals', label: '待处理申诉', count: pendingAppeals, to: '/school-admin/appeals' },
          { key: 'alerts', label: '待处理告警', count: pendingAlerts, to: '/school-admin/system-alerts' },
        ]}
      />

      <StatisticsTrendSection
        load={(range) => api.admin.getSchoolStatistics(range)}
        idPrefix="school"
        description="按日聚合的历史快照;每张图都可以切到数据表看逐日数字。"
      />
    </PageScaffold>
  )
}
