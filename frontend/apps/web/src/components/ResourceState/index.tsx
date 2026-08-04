// ResourceState 统一渲染页面级资源的加载、空态与错误态(规范 §6.5 三态 / §6.7 展示位 A)。
// 各页面自己写这三段会导致同一种失败在不同页面长得不一样,也容易漏掉报障编号,
// 故收敛到一处:骨架预留空间防布局跳动、空态给引导与行动、错误态可重试并只显示后端 trace_id。

import type { ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'
import { Button, Empty, Skeleton } from '@chaimir/ui'
import type { AsyncResourceState } from '../../hooks/useAsyncResource'
import { RESOURCE_LOAD_FAILED_MESSAGE } from '../../utils/userFacingError'

export interface ResourceStateProps<T> {
  /** useAsyncResource 的返回值 */
  resource: AsyncResourceState<T>
  /** 空态图标(Lucide) */
  emptyIcon: LucideIcon
  /** 空态标题:说明这里本该有什么 */
  emptyTitle: string
  /** 空态说明:为什么是空的、可以做什么 */
  emptyDescription: string
  /** 空态行动按钮(可选,如「用邀请码加入课程」) */
  emptyAction?: ReactNode
  /** 加载骨架:默认按列表形态渲染,复杂页面可传入与真实内容同形的骨架 */
  skeleton?: ReactNode
  /** 数据就绪后的渲染;仅在 success 时调用 */
  children: (data: T) => ReactNode
}

/**
 * ResourceState 按资源状态选择渲染分支。
 * 错误文案取后端 message(后端已是用户向),非 ApiError 已在 useAsyncResource 边界处
 * 换成用户向文案;编号只在后端签发 trace_id 时展示,前端不自造。
 */
export function ResourceState<T>({
  resource,
  emptyIcon,
  emptyTitle,
  emptyDescription,
  emptyAction,
  skeleton,
  children,
}: ResourceStateProps<T>) {
  if (resource.status === 'loading') {
    return (
      <>
        {skeleton ?? (
          <div className="flex flex-col gap-3">
            <Skeleton variant="block" />
            <Skeleton variant="line" lines={3} />
          </div>
        )}
      </>
    )
  }

  if (resource.status === 'error') {
    return (
      <ResourceError
        message={resource.error?.message ?? RESOURCE_LOAD_FAILED_MESSAGE}
        traceId={resource.error?.traceId}
        onRetry={resource.reload}
      />
    )
  }

  if (resource.status === 'empty') {
    return (
      <Empty
        icon={emptyIcon}
        title={emptyTitle}
        description={emptyDescription}
        action={emptyAction}
      />
    )
  }

  return <>{resource.data === null ? null : children(resource.data)}</>
}

export interface ResourceErrorProps {
  /** 用户向失败文案 */
  message: string
  /** 后端签发的报障编号;不存在时不显示编号 */
  traceId?: string
  onRetry: () => void
}

/**
 * ResourceError 渲染可恢复的读取失败态。
 * 单独导出供并列资源区(同页多块数据)复用,避免各区自行拼错误块。
 */
export function ResourceError({ message, traceId, onRetry }: ResourceErrorProps) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-lg border border-line bg-surface px-6 py-12 text-center">
      <p className="text-base text-ink">{message}</p>
      {traceId ? (
        <p className="font-mono text-xs text-ink-faint">如需帮助,请提供编号 {traceId}</p>
      ) : null}
      <Button variant="outline" onClick={onRetry}>
        重新加载
      </Button>
    </div>
  )
}
