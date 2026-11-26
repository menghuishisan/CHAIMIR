// 全平台运营概览、待办与趋势；不是登录落地页。

import { LayoutDashboard } from 'lucide-react'
import { AlertStatus, RuntimeSelftestStatus } from '@chaimir/api-client'
import { PageHeader, PageScaffold, Skeleton } from '@chaimir/ui'
import { api } from '../../../../app/api'
import { ResourceState } from '../../../../components/ResourceState'
import { useAsyncResource, useResourceTotal } from '../../../../hooks'
import { DashboardAttention, DashboardGeneratedNote, DashboardKeyFigures } from '../../components/DashboardSections'
import { StatisticsTrendSection } from '../../components/StatisticsTrendSection'

/**
 * PlatformDashboardPage 呈现全平台概览、需要处理的事与运营趋势。
 */
export default function PlatformDashboardPage() {
  const dashboard = useAsyncResource(() => api.admin.getPlatformDashboard(), [], () => false)
  const pendingAlerts = useResourceTotal(
    (params) => api.admin.listAlertEvents({ status: AlertStatus.PENDING, ...params }),
    [],
    dashboard.refreshVersion,
  )
  // 运行时目录不分页，按完整结果统计自检失败数量。
  const runtimes = useAsyncResource(() => api.sandbox.listRuntimes(), [dashboard.refreshVersion], () => false)
  const failedRuntimes =
    runtimes.status === 'success'
      ? (runtimes.data ?? []).filter((runtime) => runtime.selftest_status === RuntimeSelftestStatus.FAILED).length
      : undefined

  return (
    <PageScaffold>
      <PageHeader
        title="平台看板"
        actions={dashboard.status === 'success' && dashboard.data ? <DashboardGeneratedNote generatedAt={dashboard.data.generated_at} /> : undefined}
      />

      <ResourceState
        resource={dashboard}
        emptyIcon={LayoutDashboard}
        emptyTitle="暂无看板数据"
        emptyDescription="学校开始使用后会在这里显示概览。"
        skeleton={
          <div className="metric-band mb-6 gap-4" aria-busy="true">
            {Array.from({ length: 7 }, (_, index) => (
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
          {
            key: 'applications',
            label: '待审核入驻申请',
            count: dashboard.status === 'success' ? (dashboard.data?.pending_apply_count ?? 0) : undefined,
            to: '/platform-admin/applications',
          },
          { key: 'alerts', label: '待处理告警', count: pendingAlerts, to: '/platform-admin/alerts' },
          { key: 'runtimes', label: '自检未通过的运行时', count: failedRuntimes, to: '/platform-admin/runtimes' },
        ]}
      />

      <StatisticsTrendSection
        load={(range) => api.admin.getPlatformStatistics(range)}
        idPrefix="platform"
        description="全平台按日聚合的历史快照;每张图都可以切到数据表看逐日数字。"
      />
    </PageScaffold>
  )
}
