// DashboardSections 统一 admin 两种范围看板中同形的教学实践、资源配额与快照时间区块。
// 数据来自强类型 Dashboard 契约,不再按未知 JSON 键动态猜测标签。

import type { ReactNode } from 'react'
import { Activity, Book, FlaskConical, Swords } from 'lucide-react'
import type { Dashboard, ResourceQuotaSnapshot } from '@chaimir/api-client'
import { DescriptionList, PageSection, Stat } from '@chaimir/ui'
import { formatDateTime } from '../../../utils/formatters'

interface DashboardTeachingPracticeSectionProps {
  dashboard: Dashboard
}

/** DashboardTeachingPracticeSection 展示两个管理范围共用的课程、实验、竞赛与沙箱指标。 */
export function DashboardTeachingPracticeSection({ dashboard }: DashboardTeachingPracticeSectionProps) {
  return (
    <PageSection title="教学与实践" description="课程、实验与竞赛的开展情况。">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Stat label="课程" value={dashboard.course_count} icon={Book} hint={`进行中 ${dashboard.active_course_count}`} />
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
  )
}

interface DashboardQuotaSectionProps {
  snapshot?: ResourceQuotaSnapshot
  title: string
  description: string
}

/** DashboardQuotaSection 按公开 DTO 的固定字段展示资源上限。 */
export function DashboardQuotaSection({ snapshot, title, description }: DashboardQuotaSectionProps) {
  if (!snapshot) return null

  return (
    <PageSection title={title} description={description}>
      <DescriptionList
        dense
        columns={2}
        items={[
          { term: '实验环境并发上限', description: String(snapshot.max_concurrent_sandbox), mono: true },
          { term: 'CPU 上限', description: String(snapshot.max_cpu), mono: true },
          { term: '内存上限(MB)', description: String(snapshot.max_memory_mb), mono: true },
        ]}
      />
    </PageSection>
  )
}

interface DashboardGeneratedFooterProps {
  generatedAt: string
  children: ReactNode
}

/** DashboardGeneratedFooter 展示快照时间和各范围自己的快捷入口。 */
export function DashboardGeneratedFooter({ generatedAt, children }: DashboardGeneratedFooterProps) {
  return (
    <PageSection>
      <div className="flex flex-wrap items-center justify-between gap-3 well p-4">
        <div className="min-w-0">
          <p className="text-sm text-ink">数据生成于 {formatDateTime(generatedAt)}</p>
          <p className="text-xs text-ink-sub">概览按周期聚合,不是实时值。需要看实时状态请去对应功能页。</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">{children}</div>
      </div>
    </PageSection>
  )
}
