/**
 * Card:白卡(规范 §6.5.1):`bg-surface` + `shadow-xs` + `radius-lg`,不画边框。
 *
 * 整卡可点(interactive):卡内放且只放一个带 `card-link` 类的真实链接或按钮,它的 ::after 铺满整张卡
 * (见 tokens/theme.css 的 interactive-card / card-link)—— 点卡片任何位置都是在点这个链接,
 * 中键与 Ctrl/⌘ 点击在新标签页打开、右键能复制链接、读屏读到的是链接文字。
 * 卡内其它按钮加 `relative z-base` 浮在覆盖层之上,单独可点、不触发整卡。
 * 不再用 role="button" + onClick 模拟整卡点击:那样做不了新标签页打开,也没有可复制的链接。
 */
import type { HTMLAttributes, ReactNode } from 'react'
import { cn } from '../../lib/cn'

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  /** 整卡可点:卡内须有且只有一个带 card-link 类的链接或按钮 */
  interactive?: boolean
}

/** Card 渲染一张白卡;interactive 时悬停升影、键盘焦点画在整张卡上、按下有按压反馈。 */
export function Card({ interactive = false, className, ...rest }: CardProps) {
  return (
    <div
      {...rest}
      className={cn('rounded-lg bg-surface shadow-xs', interactive && 'interactive-card', className)}
    />
  )
}

export interface CardHeaderProps {
  /** 卡片标题 */
  title: ReactNode
  /** 标题旁的计数,如「近期截止 3」;未知时不传,不用 0 冒充 */
  count?: number
  /** 一句说明;只在读者确实需要时给 */
  description?: ReactNode
  /** 右侧入口或操作,如「查看全部」 */
  actions?: ReactNode
  className?: string
}

/** 卡头:「标题 + 计数 + 右侧入口」,不画底线 —— 与内容之间靠间距分开(规范 §6.5.1)。 */
export function CardHeader({ title, count, description, actions, className }: CardHeaderProps) {
  return (
    <div className={cn('flex flex-wrap items-start justify-between gap-3 px-5 pb-2 pt-5', className)}>
      <div className="min-w-0">
        <div className="flex items-baseline gap-2">
          <div className="text-lg font-bold text-ink">{title}</div>
          {count !== undefined && <span className="text-sm tabular-nums text-ink-sub">{count}</span>}
        </div>
        {description !== undefined && description !== null && (
          <div className="mt-0.5 text-sm text-ink-sub">{description}</div>
        )}
      </div>
      {actions !== undefined && actions !== null && (
        <div className="flex max-w-full flex-wrap items-center gap-2 sm:shrink-0 sm:justify-end">{actions}</div>
      )}
    </div>
  )
}

export type CardBodyProps = HTMLAttributes<HTMLDivElement>

/** 卡体:统一内距 */
export function CardBody({ className, ...rest }: CardBodyProps) {
  return <div {...rest} className={cn('p-5', className)} />
}

export type CardFooterProps = HTMLAttributes<HTMLDivElement>

/** 卡脚:上边线分隔,默认操作靠右排布 */
export function CardFooter({ className, ...rest }: CardFooterProps) {
  return (
    <div
      {...rest}
      className={cn(
        'flex items-center justify-end gap-3 border-t border-line px-5 py-4',
        className
      )}
    />
  )
}
