/**
 * InteractiveLegend:图表系列的可访问切换器。
 *
 * Recharts 默认图例只提供鼠标点击回调,这里统一使用原生 button,让系列显隐
 * 同时具备键盘操作、焦点反馈与 aria-pressed 状态,避免每种图表各写一套交互。
 */
import { cn } from '../lib/cn'
import type { ChartContext } from './palette'

interface InteractiveLegendSeries {
  key: string
  name: string
}

export interface InteractiveLegendProps {
  series: InteractiveLegendSeries[]
  hiddenKeys: ReadonlySet<string>
  onToggle: (key: string) => void
  context?: ChartContext
}

/** InteractiveLegend 渲染可键盘操作的系列显示开关。 */
export function InteractiveLegend({
  series,
  hiddenKeys,
  onToggle,
  context = 'paper',
}: InteractiveLegendProps) {
  const textClass = context === 'dark' ? 'text-on-dark-sub' : 'text-ink-sub'
  const activeClass = context === 'dark' ? 'text-on-dark' : 'text-ink'
  const markerClasses =
    context === 'dark'
      ? ['bg-accent', 'bg-on-dark-amber', 'bg-on-dark-blue', 'bg-on-dark-violet']
      : ['bg-primary', 'bg-warning', 'bg-info', 'bg-success']

  return (
    <div
      role="group"
      className="flex flex-wrap justify-center gap-x-3 gap-y-1"
      aria-label="图表系列显示控制"
    >
      {series.map((seriesItem, index) => {
        const visible = !hiddenKeys.has(seriesItem.key)
        return (
          <button
            key={seriesItem.key}
            type="button"
            aria-pressed={visible}
            onClick={() => onToggle(seriesItem.key)}
            className={cn(
              'inline-flex min-h-8 items-center gap-1.5 rounded-sm px-1 text-xs outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-surface',
              context === 'dark' && 'focus-visible:ring-accent focus-visible:ring-offset-dark-bg',
              visible ? activeClass : textClass,
              !visible && 'opacity-55'
            )}
          >
            <span
              aria-hidden="true"
              className={cn('size-2 rounded-full', markerClasses[index % markerClasses.length])}
            />
            <span>{seriesItem.name}</span>
          </button>
        )
      })}
    </div>
  )
}
