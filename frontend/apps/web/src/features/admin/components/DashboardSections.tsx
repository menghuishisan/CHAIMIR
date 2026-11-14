// DashboardSections 学校看板与平台看板共用的区块(设计 §7.5 / §7.6):
// 第一行关键数字小卡(自动排列,每张一个数字 + 一句口径 + 可选的次要数字);师生构成只有两类,
// 放在数字卡里的比例条,不再画大环形图;实验环境卡显示运行中数量与并发上限的使用比例条。
// 第二行「需要处理」:每项一个数字(用已有列表的总数接口取),整项可点进对应页面。
// 数据来自强类型 Dashboard 契约,不按未知 JSON 键猜测。

import type { ReactNode } from 'react'
import { Link } from 'react-router'
import { ChevronRight } from 'lucide-react'
import type { Dashboard } from '@chaimir/api-client'
import { cn, Icon, Skeleton } from '@chaimir/ui'
import { formatDateTime, formatRelativeOccurrence } from '../../../utils/formatters'

interface KeyFigureProps {
  label: string
  value: number
  /** 一句口径(这个数字数的是什么) */
  hint: string
  /** 次要数字,如「活跃 312」 */
  secondary?: string
  /** 可选的比例条(师生构成、配额使用) */
  children?: ReactNode
}

/** KeyFigure 一张关键数字小卡:名称、大数字、次要数字、口径,可带比例条 */
function KeyFigure({ label, value, hint, secondary, children }: KeyFigureProps) {
  return (
    <div className="flex flex-col gap-1 rounded-lg bg-surface p-4 shadow-xs">
      <p className="text-sm text-ink-sub">{label}</p>
      <p className="flex flex-wrap items-baseline gap-x-2">
        <span className="text-stat font-bold tabular-nums text-ink">{value.toLocaleString('zh-CN')}</span>
        {secondary ? <span className="text-sm tabular-nums text-ink-sub">{secondary}</span> : null}
      </p>
      {children}
      <p className="text-xs text-ink-sub">{hint}</p>
    </div>
  )
}

/**
 * RatioBar 两部分的比例条(如教师 / 学生):两段用不同深浅的玉色,各带文字与人数,不只靠颜色。
 * 宽度是运行时计算的百分比,按规范允许内联 style。
 */
function RatioBar({ left, right }: { left: { label: string; value: number }; right: { label: string; value: number } }) {
  const total = left.value + right.value
  const leftShare = total > 0 ? Math.round((left.value / total) * 100) : 0
  return (
    <div className="mt-1 flex flex-col gap-1">
      <div
        role="img"
        aria-label={`${left.label} ${left.value} 人,${right.label} ${right.value} 人`}
        className="flex h-2 overflow-hidden rounded-full bg-surface-sunken"
      >
        {total > 0 ? (
          <>
            {/* 动态宽度:占比来自接口数据,属于运行时计算值 */}
            <span className="h-full bg-primary" style={{ width: `${leftShare}%` }} />
            <span className="h-full flex-1 bg-primary-soft" />
          </>
        ) : null}
      </div>
      <p className="flex justify-between text-xs tabular-nums text-ink-sub">
        <span>
          {left.label} {left.value}
        </span>
        <span>
          {right.label} {right.value}
        </span>
      </p>
    </div>
  )
}

/** UsageBar 配额使用比例条:超过 85% 用警示色 */
function UsageBar({ used, limit }: { used: number; limit: number }) {
  const share = limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : 0
  return (
    <div className="mt-1 flex flex-col gap-1">
      <div
        role="progressbar"
        aria-label="实验环境并发使用比例"
        aria-valuenow={share}
        aria-valuemin={0}
        aria-valuemax={100}
        className="h-2 overflow-hidden rounded-full bg-surface-sunken"
      >
        {/* 动态宽度:使用比例来自接口数据,属于运行时计算值 */}
        <span className={cn('block h-full', share >= 85 ? 'bg-warning' : 'bg-primary')} style={{ width: `${share}%` }} />
      </div>
      <p className="text-xs tabular-nums text-ink-sub">
        并发上限 {limit},已用 {share}%
      </p>
    </div>
  )
}

/**
 * DashboardKeyFigures 关键数字小卡一行:账号(总数 / 活跃)、师生(教师 / 学生,比例条)、课程(进行中 / 总数)、
 * 实验(数量 / 运行中实例)、竞赛(进行中 / 总数)、实验环境(运行中 · 配额使用比例条);平台范围多一张「学校」。
 */
export function DashboardKeyFigures({ dashboard }: { dashboard: Dashboard }) {
  const quota = dashboard.resource_quota_snapshot
  return (
    <section aria-label="关键数字" className="metric-band mb-6 gap-4">
      {dashboard.tenant_count !== undefined ? <KeyFigure label="学校" value={dashboard.tenant_count} hint="已开通的学校" /> : null}
      <KeyFigure
        label="账号"
        value={dashboard.account_count}
        secondary={`活跃 ${dashboard.active_account_count}`}
        hint="含管理员;活跃指近期有登录"
      />
      <KeyFigure label="师生" value={dashboard.teacher_count + dashboard.student_count} hint="教师与学生合计,不含管理员">
        <RatioBar left={{ label: '教师', value: dashboard.teacher_count }} right={{ label: '学生', value: dashboard.student_count }} />
      </KeyFigure>
      <KeyFigure label="课程" value={dashboard.course_count} secondary={`进行中 ${dashboard.active_course_count}`} hint="全部课程" />
      <KeyFigure
        label="实验"
        value={dashboard.experiment_count}
        secondary={`运行中实例 ${dashboard.active_instance_count}`}
        hint="已编排的实验"
      />
      <KeyFigure label="竞赛" value={dashboard.contest_count} secondary={`进行中 ${dashboard.active_contest_count}`} hint="全部赛事" />
      <KeyFigure label="实验环境" value={dashboard.active_sandbox_count} hint="正在占用运行资源的环境">
        {quota && quota.max_concurrent_sandbox > 0 ? <UsageBar used={dashboard.active_sandbox_count} limit={quota.max_concurrent_sandbox} /> : null}
      </KeyFigure>
    </section>
  )
}

/** 「需要处理」的一项:数字(读取中为 undefined)、说法与去处 */
export interface AttentionItem {
  key: string
  label: string
  count: number | undefined
  to: string
}

/**
 * DashboardAttention 「需要处理」一行:每项一个数字,整项是真实链接;数字大于 0 时用警示色,
 * 为 0 时写「没有待处理」;读取中显示占位。
 */
export function DashboardAttention({ items }: { items: AttentionItem[] }) {
  return (
    <section aria-labelledby="dashboard-attention-title" className="mb-8">
      <h2 id="dashboard-attention-title" className="mb-3 text-md font-semibold text-ink">
        需要处理
      </h2>
      <ul className="card-grid gap-4">
        {items.map((item) => (
          <li key={item.key}>
            <Link
              to={item.to}
              className="group flex h-full items-center justify-between gap-3 rounded-lg bg-surface p-4 shadow-xs transition-shadow duration-fast hover:shadow-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
            >
              <span className="min-w-0">
                <span className="block text-sm text-ink-sub">{item.label}</span>
                {item.count === undefined ? (
                  <Skeleton variant="line" width="half" className="mt-2" />
                ) : (
                  <span className={cn('mt-1 block text-stat font-bold tabular-nums', item.count > 0 ? 'text-warning' : 'text-ink')}>
                    {item.count}
                  </span>
                )}
                {item.count === 0 ? <span className="block text-xs text-ink-sub">没有待处理</span> : null}
              </span>
              <Icon icon={ChevronRight} size="sm" className="shrink-0 text-ink-faint group-hover:text-primary" />
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}

/** DashboardGeneratedNote 概览的快照时间:概览按周期聚合,不是实时值 */
export function DashboardGeneratedNote({ generatedAt }: { generatedAt: string }) {
  return (
    <span className="text-sm text-ink-sub" title={formatDateTime(generatedAt)}>
      概览生成于 {formatRelativeOccurrence(generatedAt)},不是实时值
    </span>
  )
}
