// AppStatusScreen 是底层(墨色)语境的全屏/区块状态屏:
// 供路由守卫校验、403/404、页面出错等壳层场景复用,统一图标 + 标题 + 说明 + 动作的层级。

import React from 'react'
import type { LucideIcon } from 'lucide-react'
import { Icon, cn } from '@chaimir/ui'
import { traceHintText } from '../../utils/userFacingError'

/** 状态语气:accent 用于进行中/中性状态,danger 用于失败(错误不用品牌色,规范 §6.7 D)。 */
export type AppStatusTone = 'accent' | 'danger'

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
}

/** 语气 → 图标色:深底错误文字用独立冷红令牌(对墨底 ≈8.9:1),不复用朱砂。 */
const TONE_ICON_CLASS: Record<AppStatusTone, string> = {
  accent: 'text-accent',
  danger: 'text-on-dark-danger',
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
}: AppStatusScreenProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center bg-substrate px-6 py-16 text-center text-on-dark',
        // 嵌入沉浸壳时必须占满剩余弹性空间:否则容器只有内容高,justify-center 无从生效,
        // 状态屏会贴在屏幕顶部、下方留一大片空墨底。
        fullScreen ? 'min-h-dvh' : 'min-h-0 flex-1',
      )}
    >
      <div className="flex h-14 w-14 items-center justify-center rounded-lg border border-dark-line bg-dark-surface">
        <Icon
          icon={icon}
          size="lg"
          className={cn(TONE_ICON_CLASS[tone], spinning && 'animate-spin')}
        />
      </div>
      <h1 className="mt-5 text-xl font-bold">{title}</h1>
      {description ? (
        <p className="mt-2 max-w-sm text-sm text-on-dark-sub">{description}</p>
      ) : null}
      {traceId ? (
        <p className="mt-3 font-mono text-xs text-on-dark-sub">{traceHintText(traceId)}</p>
      ) : null}
      {actions ? <div className="mt-6 flex items-center gap-3">{actions}</div> : null}
    </div>
  )
}
