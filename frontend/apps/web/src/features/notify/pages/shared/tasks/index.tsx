// TasksPage 展示统一导入导出任务中心，数据来自 platform/transfer 后端模块。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError, TransferTask } from '@chaimir/api-client'
import { Button } from '@chaimir/ui'
import { Download, Loader, RefreshCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { EmptyState, ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../shared.module.css'
import { formatDateTime, transferTaskStatusLabel } from '../../../../../utils/index'

const PAGE_SIZE = 20


/**
 * taskProgress 根据任务状态给出稳定进度展示，真实完成状态仍以后端 status 为准。
 */
function taskProgressClass(status: TransferTask['status']): string {
  if (status === 'succeeded') {
    return styles.progressDone
  }
  if (status === 'failed') {
    return styles.progressDone
  }
  if (status === 'running') {
    return styles.progressRunning
  }
  if (status === 'retrying') {
    return styles.progressRetrying
  }
  return styles.progressPending
}


/**
 * TasksPage 读取当前账号可见的导入导出任务并签发下载授权。
 */
const TasksPage: React.FC = () => {
  const resource = useAsyncResource(
    () => api.transfer.listTasks({ page: 1, size: PAGE_SIZE }),
    []
  )
  const [grantByTask, setGrantByTask] = useState<Record<string, string>>({})
  const [grantError, setGrantError] = useState<ApiError | null>(null)
  const [grantingTaskId, setGrantingTaskId] = useState<string | null>(null)

  const tasks = useMemo(() => resource.data?.items || [], [resource.data])

  const handleGrant = useCallback(async (task: TransferTask) => {
    setGrantError(null)
    setGrantingTaskId(task.task_id)
    try {
      const grant = await api.transfer.downloadGrant(task.task_id)
      setGrantByTask((current) => ({
        ...current,
        [task.task_id]: `下载授权已签发，有效期至 ${formatDateTime(grant.expires_at)}`,
      }))
    } catch (error) {
      setGrantError(error as ApiError)
    } finally {
      setGrantingTaskId(null)
    }
  }, [])

  return (
    <div className={styles.page}>
      <div className={styles.breadcrumb}>共享功能 / 任务中心</div>

      <div className={styles.header}>
        <h1 className={styles.title}>
          <Loader className={styles.titleIcon} size={28} />
          异步任务与下载中心
        </h1>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>
          刷新
        </Button>
      </div>

      {grantError && (
        <ErrorState error={grantError} onRetry={() => setGrantError(null)} title="下载授权签发失败" />
      )}

      {resource.status === 'loading' && <LoadingState title="正在获取任务" />}
      {resource.status === 'error' && (
        <ErrorState error={resource.error} onRetry={resource.reload} />
      )}
      {resource.status === 'empty' && (
        <EmptyState title="暂无任务" description="当前账号还没有导入、导出或下载任务。" />
      )}
      {resource.status === 'success' && (
        <div className={styles.list}>
          {tasks.map((task) => (
            <article className={styles.card} key={task.task_id}>
              <div className={styles.cardMain}>
                <h2 className={styles.cardTitle}>{task.subject}</h2>
                <div className={styles.meta}>
                  <span className={styles.status}>{transferTaskStatusLabel(task.status)}</span>
                  <span>{task.channel === 'export' ? '导出任务' : '导入任务'}</span>
                  <span>更新时间 {formatDateTime(task.updated_at)}</span>
                  {task.artifact_file_name && <span>{task.artifact_file_name}</span>}
                </div>
                <div className={styles.progressTrack} aria-hidden="true">
                  <div className={`${styles.progressFill} ${taskProgressClass(task.status)}`} />
                </div>
                {grantByTask[task.task_id] && (
                  <div className={styles.grant}>{grantByTask[task.task_id]}</div>
                )}
              </div>
              <Button
                variant="primary"
                icon={<Download size={16} />}
                loading={grantingTaskId === task.task_id}
                disabled={task.status !== 'succeeded'}
                onClick={() => handleGrant(task)}
              >
                获取下载授权
              </Button>
            </article>
          ))}
        </div>
      )}
    </div>
  )
}

export default TasksPage
