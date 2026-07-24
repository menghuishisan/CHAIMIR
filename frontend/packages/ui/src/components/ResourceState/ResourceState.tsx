// ResourceState 统一呈现页面区域的加载、错误、空数据、无权限和正常五态。

import React from 'react'
import { AlertTriangle, Inbox, LockKeyhole, RefreshCw } from 'lucide-react'
import { clsx } from 'clsx'
import { Button } from '../Button'
import { Empty } from '../Empty'
import { Spinner } from '../Spinner'
import './ResourceState.css'

export type ResourceStatus = 'loading' | 'error' | 'empty' | 'forbidden' | 'ready'

export interface ResourceError {
  message?: string
  traceId?: string
  status?: number
}

export interface ResourceStateProps {
  status: ResourceStatus
  title?: string
  description?: string
  error?: ResourceError | null
  onRetry?: () => void
  actionLabel?: string
  action?: React.ReactNode
  children?: React.ReactNode
  className?: string
}

/**
 * ResourceState 将终端用户可见的资源状态限制在统一的五态契约内。
 */
export function ResourceState({
  status,
  title,
  description,
  error,
  onRetry,
  actionLabel = '重试',
  action,
  children,
  className,
}: ResourceStateProps): React.ReactElement | null {
  if (status === 'ready') return <>{children}</>

  const isForbidden = status === 'forbidden' || (status === 'error' && error?.status === 403)

  if (status === 'loading') {
    return (
      <div className={clsx('chaimir-resource-state', 'chaimir-resource-state--loading', className)} role="status">
        <Spinner label={title || '正在获取数据'} />
        <p>{description || '请稍候，系统正在同步最新信息。'}</p>
      </div>
    )
  }

  if (status === 'empty') {
    return (
      <Empty
        icon={<Inbox size={32} />}
        title={title || '暂无数据'}
        description={description || '当前还没有可展示的记录。'}
        action={action}
        className={clsx('chaimir-resource-state--empty', className)}
      />
    )
  }

  if (isForbidden) {
    return (
      <div className={clsx('chaimir-resource-state', 'chaimir-resource-state--forbidden', className)} role="status">
        <LockKeyhole size={28} aria-hidden="true" />
        <h3>{title || '暂无访问权限'}</h3>
        <p>{description || '请联系管理员确认你的访问范围。'}</p>
        {action && <div className="chaimir-resource-state__action">{action}</div>}
      </div>
    )
  }

  return (
    <div className={clsx('chaimir-resource-state', 'chaimir-resource-state--error', className)} role="alert">
      <div className="chaimir-resource-state__icon" aria-hidden="true">
        <AlertTriangle size={22} />
      </div>
      <div className="chaimir-resource-state__body">
        <h3>{title || '暂时无法获取数据'}</h3>
        <p>{description || error?.message || '请稍后重试。'}</p>
        {error?.traceId && <span className="chaimir-resource-state__trace">如需帮助，请提供编号 {error.traceId}</span>}
      </div>
      {onRetry && (
        <Button variant="outline" size="sm" icon={<RefreshCw size={16} />} onClick={onRetry}>
          {actionLabel}
        </Button>
      )}
    </div>
  )
}

ResourceState.displayName = 'ResourceState'
