// AnswerSheet 答题卡(设计 §7.2 作业作答):每题一个格子,已作答为玉色实底,当前题描边,点格子跳到那题。
// ≥lg 是左侧吸顶的方格网;更窄时是题目区上方一行可横向滚动的格子。
// 「当前题」由页面按滚动位置给出(hooks/useActiveAnchor),点格子时立即更新。

import { cn } from '@chaimir/ui'

export interface AnswerSheetItem {
  id: string
  seq: number
  answered: boolean
}

export interface AnswerSheetProps {
  items: AnswerSheetItem[]
  currentId: string | undefined
  onJump: (itemId: string) => void
  className?: string
}

/** AnswerSheet 渲染答题卡 */
export function AnswerSheet({ items, currentId, onJump, className }: AnswerSheetProps) {
  const answered = items.filter((item) => item.answered).length
  return (
    <nav aria-label="答题卡" className={cn('rounded-lg bg-surface p-4 shadow-xs', className)}>
      <div className="mb-3 flex items-baseline justify-between gap-2">
        <span className="text-sm font-semibold text-ink">答题卡</span>
        <span className="text-xs tabular-nums text-ink-sub">
          已作答 {answered} / {items.length}
        </span>
      </div>
      <ol className="flex gap-2 overflow-x-auto p-1 lg:grid lg:grid-cols-5 lg:overflow-visible">
        {items.map((item) => {
          const current = item.id === currentId
          return (
            <li key={item.id} className="shrink-0">
              <button
                type="button"
                onClick={() => onJump(item.id)}
                aria-label={`第 ${item.seq} 题,${item.answered ? '已作答' : '未作答'}`}
                aria-current={current ? 'step' : undefined}
                className={cn(
                  'flex size-10 items-center justify-center rounded-md text-sm font-medium tabular-nums transition-colors duration-fast',
                  'lg:aspect-square lg:size-auto lg:w-full',
                  'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent',
                  item.answered
                    ? 'bg-primary text-on-solid hover:bg-primary-hover'
                    : 'bg-surface-hover text-ink-sub hover:text-ink',
                  current && 'ring-2 ring-primary ring-offset-2 ring-offset-surface'
                )}
              >
                {item.seq}
              </button>
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
