// ContestTimeline 赛程条(设计 §7.2 学生竞赛详情 / 教师赛事详情):报名开始 → 报名截止 → 开赛 → 封榜 → 结束,
// 当前阶段高亮。节点与当前阶段由 contestTimeline 计算(带测试),这里只管呈现。
// 宽屏横排一条线,窄屏(< sm)改竖排,每个节点都写出日期与时刻;状态同时用文字表达(读屏可读)。

import type { Contest } from '@chaimir/api-client'
import { cn } from '@chaimir/ui'
import { formatShortDateTime } from '../../../utils/formatters'
import { contestTimeline } from '../contestTimeline'

const STATE_TEXT = { done: '已过', current: '当前阶段', upcoming: '未到' } as const

/** ContestTimeline 渲染一场赛事的赛程条 */
export function ContestTimeline({ contest, className }: { contest: Contest; className?: string }) {
  const nodes = contestTimeline(contest)
  return (
    <ol
      aria-label="赛程"
      className={cn('flex flex-col gap-3 rounded-lg bg-surface p-4 shadow-xs sm:flex-row sm:gap-0 sm:p-5', className)}
    >
      {nodes.map((node, index) => (
        <li
          key={node.key}
          aria-current={node.state === 'current' ? 'step' : undefined}
          className="relative flex min-w-0 flex-1 items-start gap-3 sm:flex-col sm:items-center sm:gap-2 sm:text-center"
        >
          {/* 连线:宽屏画在节点右侧,已过的一段用玉色 */}
          {index < nodes.length - 1 ? (
            <span
              aria-hidden="true"
              className={cn(
                'absolute left-1/2 top-2 hidden h-0.5 w-full sm:block',
                node.state === 'done' ? 'bg-primary' : 'bg-line',
              )}
            />
          ) : null}
          <span
            aria-hidden="true"
            className={cn(
              'relative z-base mt-0.5 size-4 shrink-0 rounded-full border-2 sm:mt-0',
              node.state === 'done' && 'border-primary bg-primary',
              node.state === 'current' && 'border-primary bg-surface ring-4 ring-primary-soft',
              node.state === 'upcoming' && 'border-line-strong bg-surface',
            )}
          />
          <span className="min-w-0">
            <span
              className={cn(
                'block text-sm',
                node.state === 'current' ? 'font-semibold text-primary' : node.state === 'done' ? 'text-ink' : 'text-ink-sub',
              )}
            >
              {node.label}
              <span className="sr-only">,{STATE_TEXT[node.state]}</span>
            </span>
            <span className="block font-mono text-xs tabular-nums text-ink-sub">{formatShortDateTime(node.at)}</span>
          </span>
        </li>
      ))}
    </ol>
  )
}
