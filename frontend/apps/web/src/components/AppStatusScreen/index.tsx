// AppStatusScreen 是壳层场景的全屏/区块状态屏:
// 供路由守卫校验、403/404、页面出错等场景复用,统一图标 + 标题 + 说明 + 动作的层级。
// 默认铺在墨色底层(没有壳、或沉浸壳);日常壳里只换掉内容区时铺在宣纸纸面上,导航与顶栏照常留着。

import React from 'react'
import type { LucideIcon } from 'lucide-react'
import { Icon, cn } from '@chaimir/ui'
import { traceHintText } from '../../utils/userFacingError'

/** 状态语气:accent 用于进行中/中性状态,danger 用于失败(错误不用品牌色,规范 §6.7 D)。 */
type AppStatusTone = 'accent' | 'danger'

/** 状态屏所在的层:substrate 墨色底层;paper 日常壳的内容纸面 */
export type AppStatusSurface = 'substrate' | 'paper'

export interface AppStatusScreenProps {
  icon: LucideIcon
  /** 图标是否旋转(仅用于「正在进行」类状态,属加载指示) */
  spinning?: boolean
  /** 语气,默认中性;失败态必须传 danger —— 玉色图标配「没能打开」是语义错位 */
  tone?: AppStatusTone
  title: string
  description?: string
  /** 报障编号:仅在后端提供 trace_id 时展示 */
  traceId?: string
  /** 动作插槽(按钮/链接) */
  actions?: React.ReactNode
  /** 默认占满视口;嵌入在其他布局(如沉浸壳)内时关闭,此时填满父级弹性空间以保持居中 */
  fullScreen?: boolean
  /** 所在的层,默认墨色底层;铺在纸面上时用纸面的文字与图标色 */
  surface?: AppStatusSurface
}

/** 语气 → 图标色:错误一律用独立冷红令牌(墨底用深底版本,对墨底 ≈8.9:1),不复用朱砂。 */
const TONE_ICON_CLASS: Record<AppStatusSurface, Record<AppStatusTone, string>> = {
  substrate: { accent: 'text-accent', danger: 'text-on-dark-danger' },
  paper: { accent: 'text-primary', danger: 'text-danger' },
}

/** 各层的底色与正文色、图标底盘、说明文字色 */
const SURFACE_CLASS: Record<AppStatusSurface, { root: string; badge: string; sub: string }> = {
  substrate: { root: 'bg-substrate text-on-dark', badge: 'bg-dark-elevated', sub: 'text-on-dark-sub' },
  paper: { root: 'text-ink', badge: 'bg-surface-hover', sub: 'text-ink-sub' },
}

/**
 * AppStatusScreen 渲染居中的状态信息;文案面向用户(FE-4),技术细节不外露。
 */
export function AppStatusScreen({
  icon,
  spinning,
  tone = 'accent',
  title,
  description,
  traceId,
  actions,
  fullScreen = true,
  surface = 'substrate',
}: AppStatusScreenProps) {
  const surfaceClass = SURFACE_CLASS[surface]
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center px-6 py-16 text-center',
        surfaceClass.root,
        // 嵌入沉浸壳时必须占满剩余弹性空间:否则容器只有内容高,justify-center 无从生效,
        // 状态屏会贴在屏幕顶部、下方留一大片空墨底。
        fullScreen ? 'min-h-dvh' : 'min-h-0 flex-1',
      )}
    >
      <div className={cn('flex size-14 items-center justify-center rounded-full', surfaceClass.badge)}>
        <Icon
          icon={icon}
          size="lg"
          className={cn(TONE_ICON_CLASS[surface][tone], spinning && 'animate-spin')}
        />
      </div>
      <h1 className="mt-5 text-2xl font-bold">{title}</h1>
      {description ? (
        <p className={cn('mt-2 max-w-md text-base', surfaceClass.sub)}>{description}</p>
      ) : null}
      {traceId ? (
        <p className={cn('mt-3 font-mono text-xs', surfaceClass.sub)}>{traceHintText(traceId)}</p>
      ) : null}
      {actions ? <div className="mt-7 flex flex-wrap items-center justify-center gap-3">{actions}</div> : null}
    </div>
  )
}
