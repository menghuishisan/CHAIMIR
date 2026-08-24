// DashboardSections 统一 admin 两种范围看板中同形的教学实践、资源配额与快照时间区块。
// 数据来自强类型 Dashboard 契约,不再按未知 JSON 键动态猜测标签。

import type { ReactNode } from 'react'
import { Activity, Book, FlaskConical, Swords } from 'lucide-react'
import type { Dashboard, ResourceQuotaSnapshot } from '@chaimir/api-client'
import {
  ChartContainer,
  DescriptionList,
  PageSection,
  ShareDonutChart,
  Stat,
} from '@chaimir/ui'
import { formatDateTime } from '../../../utils/formatters'

interface DashboardCompositionSectionProps {
  dashboard: Dashboard
}

/**
 * DashboardCompositionSection 用环图呈现师生构成。
 *
 * 选型按数据形状(规范 §8.1):这两项加起来就是「师生」这个整体,问的是「谁占多数」——
 * 那是占比而不是趋势也不是维度对比,故用环图而不是再排两张 Stat 大卡。
 * 「账号总数」不作为整体:它还含管理员,教师 + 学生凑不成它,拿它当分母就是错数(§6.5.4)。
 * 两端都只有这一对可构成整体的口径,故只出这一张环图。
 */
export function DashboardCompositionSection({ dashboard }: DashboardCompositionSectionProps) {
  const total = dashboard.teacher_count + dashboard.student_count

  return (
    <PageSection title="师生构成" description="教师与学生的人数占比。管理员账号不计入这个整体。">
      <ChartContainer
        title="师生构成"
        description="环心是师生合计人数。"
        isEmpty={total === 0}
        emptyHint="还没有开通教师或学生账号。开通后这里会显示构成占比。"
        ariaSummary={`师生合计 ${total} 人:教师 ${dashboard.teacher_count} 人、学生 ${dashboard.student_count} 人。`}
        dataTable={{
          columns: ['身份', '人数'],
          rows: [
            ['教师', dashboard.teacher_count],
            ['学生', dashboard.student_count],
          ],
        }}
      >
        <ShareDonutChart
          // 按数值降序:读者从大到小扫环
          slices={[
            { key: 'student', name: '学生', value: dashboard.student_count },
            { key: 'teacher', name: '教师', value: dashboard.teacher_count },
          ]}
          totalLabel="师生合计"
        />
      </ChartContainer>
    </PageSection>
  )
}

interface DashboardTeachingPracticeSectionProps {
  dashboard: Dashboard
}

/** DashboardTeachingPracticeSection 展示两个管理范围共用的课程、实验、竞赛与沙箱指标。 */
export function DashboardTeachingPracticeSection({ dashboard }: DashboardTeachingPracticeSectionProps) {
  return (
    <PageSection title="教学与实践" description="课程、实验与竞赛的开展情况。">
      <div className="metric-band gap-4">
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
      {/* 抬起片而非凹陷井(§6.5.1):这一条直接落在光面上,井色与光面只差一档、表达不出凹陷 */}
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-surface p-4 shadow-xs">
        <div className="min-w-0">
          <p className="text-sm text-ink">数据生成于 {formatDateTime(generatedAt)}</p>
          <p className="text-xs text-ink-sub">概览按周期聚合,不是实时值。需要看实时状态请去对应功能页。</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">{children}</div>
      </div>
    </PageSection>
  )
}
