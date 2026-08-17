/**
 * TeachingFrameStream:右侧事件流 —— 一帧里全部消息/调用按时刻分组的时间序清单(规范 §7.1/§7.2 B)。
 *
 * 它是沉浸态里唯一的滚动区:无界元素(消息/调用)只在这里出现,主舞台只留有界元素,
 * 所以消息再多也不会把图形顶出视口 —— 这正是「启动仿真后要下拉才看得到消息」的解法。
 *
 * 行为:时刻标题粘在顶部,默认跟随最新;用户上滚即停止跟随并浮出「跳到最新(n)」;
 * 可切到「只看攻击与失败」。条目复用 ElementList,故与舞台清单同一套图标 + 状态词 + 按钮语义,
 * 键盘与读屏走同一条选择路径(§7.3);减弱动效偏好下跳转不做平滑滚动。
 *
 * 纯受控:不取数、不持有运行时,选中态与选择回调由页面从仿真状态传入。
 */
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ArrowDown } from 'lucide-react'
import { cn } from '../lib/cn'
import { Icon } from '../lib/icon'
import { useReducedMotion } from '../hooks/useReducedMotion'
import { Checkbox } from '../components/Checkbox'
import { frameStreamEntries, type FrameStreamEntry } from './frameStream'
import { ElementList } from './patterns/ElementList'
import type { TeachingFrame } from '@chaimir/sim-sdk'

/** 判定「已在底部」的容差:浏览器缩放与分数像素会让 scrollTop 差几个像素 */
const BOTTOM_SLACK = 24

export interface TeachingFrameStreamProps {
  frame: TeachingFrame
  selectedElementId?: string
  /** 元素选择回调;不传则条目只读(如公开回放) */
  onSelectElement?: (elementId: string) => void
  className?: string
}

/** 同一时刻的一组条目 */
interface TickGroup {
  at: number
  entries: FrameStreamEntry[]
}

/** groupByTick:相邻同时刻条目并成一组(条目已按时刻升序) */
function groupByTick(entries: FrameStreamEntry[]): TickGroup[] {
  const groups: TickGroup[] = []
  for (const entry of entries) {
    const last = groups[groups.length - 1]
    if (last && last.at === entry.at) last.entries.push(entry)
    else groups.push({ at: entry.at, entries: [entry] })
  }
  return groups
}

export function TeachingFrameStream({
  frame,
  selectedElementId,
  onSelectElement,
  className,
}: TeachingFrameStreamProps) {
  const reducedMotion = useReducedMotion()
  const scrollRef = useRef<HTMLDivElement>(null)
  const [adverseOnly, setAdverseOnly] = useState(false)
  const [following, setFollowing] = useState(true)
  // 停止跟随那一刻的条数:之后新增多少条就是「跳到最新」要提示的数量
  const [pausedAtCount, setPausedAtCount] = useState(0)

  const entries = useMemo(() => frameStreamEntries(frame), [frame])
  const adverseCount = useMemo(() => entries.filter((entry) => entry.adverse).length, [entries])
  const visible = useMemo(
    () => (adverseOnly ? entries.filter((entry) => entry.adverse) : entries),
    [adverseOnly, entries]
  )
  const groups = useMemo(() => groupByTick(visible), [visible])
  const visibleCount = visible.length
  const pending = following ? 0 : Math.max(0, visibleCount - pausedAtCount)

  /** 上滚即停止跟随,回到底部即恢复 —— 判定完全由滚动位置给出,不额外猜测用户意图 */
  const handleScroll = useCallback(() => {
    const node = scrollRef.current
    if (!node) return
    const atBottom = node.scrollHeight - node.scrollTop - node.clientHeight <= BOTTOM_SLACK
    // 离开底部时记下基线条数,之后新增多少条就提示多少条
    if (!atBottom && following) setPausedAtCount(visibleCount)
    setFollowing(atBottom)
  }, [following, visibleCount])

  // 跟随态下每有新条目就贴到底部;减弱动效偏好下直接跳,不做平滑滚动(§7.3)
  useEffect(() => {
    if (!following) return
    const node = scrollRef.current
    if (!node) return
    node.scrollTo({ top: node.scrollHeight, behavior: reducedMotion ? 'auto' : 'smooth' })
  }, [following, reducedMotion, visibleCount])

  return (
    <div className={cn('flex h-full min-h-0 flex-col', className)}>
      <div className="flex shrink-0 flex-col gap-1.5 border-b border-dark-line px-3 py-2">
        <div className="flex items-baseline justify-between gap-2">
          <h2 className="min-w-0 text-sm font-medium text-on-dark">消息流</h2>
          <span className="shrink-0 font-mono text-xs tabular-nums text-on-dark-sub">
            {entries.length} 条
          </span>
        </div>
        <Checkbox
          onDark
          className="text-xs"
          label={adverseCount > 0 ? `只看攻击与失败(${adverseCount})` : '只看攻击与失败'}
          checked={adverseOnly}
          disabled={adverseCount === 0 && !adverseOnly}
          onCheckedChange={(checked) => setAdverseOnly(checked === true)}
        />
      </div>

      <div className="relative min-h-0 flex-1">
        <div ref={scrollRef} onScroll={handleScroll} className="h-full overflow-y-auto px-2 py-2">
          {groups.length === 0 ? (
            <p className="px-1 py-4 text-xs text-on-dark-sub">
              {adverseOnly
                ? '这次推演还没有出现攻击或失败。'
                : '推演开始后,节点之间的每一条消息都会按时刻出现在这里。'}
            </p>
          ) : (
            groups.map((group) => (
              <section key={group.at} className="flex flex-col gap-1">
                <h3 className="sticky top-0 z-base bg-dark-bg py-1 font-mono text-xs tabular-nums text-on-dark-sub">
                  推演时刻 {group.at}
                </h3>
                <ElementList
                  label={`推演时刻 ${group.at} 的消息`}
                  items={group.entries}
                  selectedElementId={selectedElementId}
                  onSelectElement={onSelectElement}
                />
              </section>
            ))
          )}
        </div>

        {pending > 0 ? (
          <div className="pointer-events-none absolute inset-x-0 bottom-2 flex justify-center">
            <button
              type="button"
              onClick={() => setFollowing(true)}
              // pressable 已含颜色过渡契约(theme.css),不再叠加 transition-colors
              className="pressable pointer-events-auto flex items-center gap-1.5 rounded-full border border-accent bg-dark-elevated px-3 py-1 text-xs text-on-dark shadow-lg hover:bg-dark-surface focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2"
            >
              <Icon icon={ArrowDown} size="xs" className="text-accent" />
              跳到最新({pending})
            </button>
          </div>
        ) : null}
      </div>
    </div>
  )
}
