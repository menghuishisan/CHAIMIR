// ContestCountdown 竞赛答题顶栏的剩余时间(设计 §7.2:最后 10 分钟用警示色)。
// 每秒刷新一次;读屏不逐秒播报(那会一直打断),只在当前文字里写明「剩余」。

import { useEffect, useReducer } from 'react'
import { Timer } from 'lucide-react'
import { cn, Icon } from '@chaimir/ui'
import { contestCountdown } from '../contestTimeline'

/** ContestCountdown 显示距 endAt 的剩余时间;结束时刻读不出来时不渲染 */
export function ContestCountdown({ endAt }: { endAt: string }) {
  const [, tick] = useReducer((count: number) => count + 1, 0)
  useEffect(() => {
    const timer = window.setInterval(tick, 1000)
    return () => window.clearInterval(timer)
  }, [])

  const countdown = contestCountdown(endAt)
  if (!countdown) return null
  return (
    <span
      className={cn(
        'flex shrink-0 items-center gap-1.5 rounded-md px-2 py-1 font-mono text-sm tabular-nums',
        countdown.urgent ? 'bg-dark-elevated text-on-dark-amber' : 'text-on-dark-sub',
      )}
    >
      <Icon icon={Timer} size="sm" />
      <span className="sr-only">{countdown.over ? '比赛已到结束时间' : '剩余时间'}</span>
      {countdown.text}
    </span>
  )
}
