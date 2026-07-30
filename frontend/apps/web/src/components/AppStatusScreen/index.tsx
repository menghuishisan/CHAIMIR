// AppStatusScreen 是底层(墨色)语境的全屏/区块状态屏:
// 供路由守卫校验、403/404、页面出错等壳层场景复用,统一图标 + 标题 + 说明 + 动作的层级。

import React from 'react'
import type { LucideIcon } from 'lucide-react'
import { Icon, cn } from '@chaimir/ui'

export interface AppStatusScreenProps {
  icon: LucideIcon
  /** 图标是否旋转(仅用于「正在进行」类状态,属加载指示) */
  spinning?: boolean
  title: string
  description?: string
  /** 报障编号:仅在后端提供 trace_id 时展示 */
  traceId?: string
  /** 动作插槽(按钮/链接) */
  actions?: React.ReactNode
  /** 默认占满视口;嵌入在其他布局(如认证壳)内时关闭 */
  fullScreen?: boolean
}

/**
 * AppStatusScreen 渲染居中的状态信息;文案面向用户(FE-4),技术细节不外露。
 */
export function AppStatusScreen({
  icon,
  spinning,
  title,
  description,
  traceId,
  actions,
  fullScreen = true,
}: AppStatusScreenProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center bg-substrate px-6 py-16 text-center text-on-dark',
        fullScreen && 'min-h-dvh',
      )}
    >
      <div className="flex h-14 w-14 items-center justify-center rounded-lg border border-dark-line bg-dark-surface">
        <Icon icon={icon} size="lg" className={cn('text-accent', spinning && 'animate-spin')} />
      </div>
      <h1 className="mt-5 text-xl font-bold">{title}</h1>
      {description ? (
        <p className="mt-2 max-w-sm text-sm text-on-dark-sub">{description}</p>
      ) : null}
      {traceId ? (
        <p className="mt-3 font-mono text-xs text-on-dark-sub">
          如需帮助,请提供编号 {traceId}
        </p>
      ) : null}
      {actions ? <div className="mt-6 flex items-center gap-3">{actions}</div> : null}
    </div>
  )
}
