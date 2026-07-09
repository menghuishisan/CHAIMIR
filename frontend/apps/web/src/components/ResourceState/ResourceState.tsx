// ResourceState 提供页面级加载、错误、空数据反馈，确保所有接口状态文案一致。

import React from 'react'
import type { ApiError } from '@chaimir/api-client'
import { Button, Empty, Spinner } from '@chaimir/ui'
import { AlertTriangle, Inbox, RefreshCw } from 'lucide-react'
import styles from './ResourceState.module.css'

export interface ResourceStateProps {
  title?: string
  description?: string
}

export interface ResourceErrorStateProps extends ResourceStateProps {
  error: ApiError | null
  onRetry: () => void
}

/**
 * LoadingState 展示页面局部数据读取中的状态。
 */
export const LoadingState: React.FC<ResourceStateProps> = ({
  title = '正在获取数据',
  description = '请稍候，系统正在同步最新信息。',
}) => (
  <div className={styles.stateCard}>
    <Spinner label={title} />
    <p>{description}</p>
  </div>
)

/**
 * EmptyState 展示后端返回空列表或空资源时的状态。
 */
export const EmptyState: React.FC<ResourceStateProps> = ({
  title = '暂无数据',
  description = '当前还没有可展示的记录。',
}) => (
  <Empty
    icon={<Inbox size={32} />}
    title={title}
    description={description}
    className={styles.empty}
  />
)

/**
 * ErrorState 展示后端或网络错误，并保留 trace_id 供报障使用。
 */
export const ErrorState: React.FC<ResourceErrorStateProps> = ({
  error,
  onRetry,
  title = '暂时无法获取数据',
  description,
}) => (
  <div className={styles.errorCard} role="alert">
    <div className={styles.errorIcon} aria-hidden="true">
      <AlertTriangle size={22} />
    </div>
    <div className={styles.errorBody}>
      <h3>{title}</h3>
      <p>{description || error?.message || '请稍后重试。'}</p>
      {error?.traceId && (
        <span className={styles.trace}>如需帮助，请提供编号 {error.traceId}</span>
      )}
    </div>
    <Button variant="outline" size="sm" icon={<RefreshCw size={16} />} onClick={onRetry}>
      重试
    </Button>
  </div>
)
