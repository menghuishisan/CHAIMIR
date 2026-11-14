// TopBarTrail 顶栏左侧的「当前位置」(规范 §6.2)。
// 宽屏:上级 › 上级 › 当前,上级可点;窄屏:返回上级按钮 + 当前页名(只留一行,不挤成一串)。
// 对象名还在读取时显示占位条;超长名称截断,完整名称放在 title 里。

import { Link } from 'react-router'
import { ArrowLeft, ChevronRight } from 'lucide-react'
import { Icon, cn } from '@chaimir/ui'
import type { PageLocationItem } from '../../hooks/pageLocation'

export interface TopBarTrailProps {
  trail: PageLocationItem[]
  /** 窄屏形态:只显示返回上级与当前页名 */
  compact: boolean
  /** 上级链接要带上的查询串(列表最近一次的筛选与页码),由外框记忆 */
  hrefOf: (to: string) => string
}

/** TrailLabel 渲染一层的文字;名称未知时是深色占位条 */
function TrailLabel({ label, className }: { label: string | undefined; className?: string }) {
  if (label === undefined) {
    return (
      <span aria-hidden="true" className="inline-block h-3.5 w-24 animate-pulse rounded-sm bg-dark-elevated" />
    )
  }
  return (
    <span title={label} className={cn('truncate', className)}>
      {label}
    </span>
  )
}

/** TopBarTrail 渲染顶栏位置;没有任何层级时返回空占位,保持顶栏右侧对齐 */
export function TopBarTrail({ trail, compact, hrefOf }: TopBarTrailProps) {
  if (trail.length === 0) return <span className="min-w-0 flex-1" />
  const current = trail[trail.length - 1]
  const parents = trail.slice(0, -1)
  const parent = [...parents].reverse().find((item) => item.to)

  if (compact) {
    return (
      <div className="flex min-w-0 flex-1 items-center gap-1">
        {parent?.to ? (
          <Link
            to={hrefOf(parent.to)}
            aria-label={parent.label ? `返回${parent.label}` : '返回上一级'}
            className="hit-target inline-flex size-9 shrink-0 items-center justify-center rounded-md text-on-dark-sub transition-colors duration-fast hover:bg-dark-elevated hover:text-on-dark"
          >
            <Icon icon={ArrowLeft} size="md" />
          </Link>
        ) : null}
        <h2 className="min-w-0 truncate text-base font-semibold text-on-dark">
          <TrailLabel label={current?.label} />
        </h2>
      </div>
    )
  }

  return (
    <nav aria-label="当前位置" className="min-w-0 flex-1">
      <ol className="flex min-w-0 items-center gap-1.5 text-sm">
        {parents.map((item, index) => (
          <li key={`${item.to ?? 'crumb'}-${index}`} className="flex min-w-0 shrink items-center gap-1.5">
            {item.to ? (
              <Link
                to={hrefOf(item.to)}
                className="min-w-0 truncate rounded-sm text-on-dark-sub transition-colors duration-fast hover:text-on-dark"
              >
                <TrailLabel label={item.label} />
              </Link>
            ) : (
              <TrailLabel label={item.label} className="text-on-dark-sub" />
            )}
            <Icon icon={ChevronRight} size="sm" className="shrink-0 text-on-dark-faint" />
          </li>
        ))}
        <li aria-current="page" className="flex min-w-0 items-center font-medium text-on-dark">
          <TrailLabel label={current?.label} />
        </li>
      </ol>
    </nav>
  )
}
