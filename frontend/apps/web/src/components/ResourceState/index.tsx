import type { ReactNode } from 'react'
import type { LucideIcon } from 'lucide-react'
import { Button, Empty, Skeleton } from '@chaimir/ui'
import type { AsyncResourceState } from '../../hooks/useAsyncResource'
import { RESOURCE_LOAD_FAILED_MESSAGE, traceHintText } from '../../utils/userFacingError'

export interface ResourceStateProps<T> {
  /** 只消费资源展示状态,刷新次数由列表与统计自行绑定。 */
  resource: Pick<AsyncResourceState<T>, 'status' | 'data' | 'error' | 'reload'>
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
 * ResourceState 展示加载、空数据、失败和成功结果。
 * 资源边界已转换用户向错误文案；报障编号只展示后端签发的 trace_id。
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
  /** 沉浸态侧栏使用深色文字和紧凑间距。 */
  onDark?: boolean
}

/** ResourceError 展示读取失败原因、服务端报障编号和重试入口。 */
export function ResourceError({ message, traceId, onRetry, onDark = false }: ResourceErrorProps) {
  return (
    <div
      role="alert"
      aria-live="assertive"
      className={onDark ? 'flex flex-col items-center gap-2 py-3 text-center' : 'flex flex-col items-center gap-3 px-6 py-12 text-center'}
    >
      <p className={onDark ? 'text-sm text-on-dark' : 'text-base text-ink'}>{message}</p>
      {traceId ? (
        <p className={'break-all font-mono text-xs ' + (onDark ? 'text-on-dark-sub' : 'text-ink-sub')}>{traceHintText(traceId)}</p>
      ) : null}
      <Button variant={onDark ? 'on-dark' : 'outline'} onClick={onRetry}>
        重新加载
      </Button>
    </div>
  )
}
