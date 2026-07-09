// TeacherMonitoringPage 展示判题任务队列，并提供重判入口。

import React, { useCallback, useMemo, useState } from 'react'
import type { ApiError, JudgeTask } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Input, Table } from '@chaimir/ui'
import { Activity, RefreshCw, RotateCw } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks'
import styles from '../../judge.module.css'

const TeacherMonitoringPage: React.FC = () => {
  const [sourceRef, setSourceRef] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const resource = useAsyncResource(() => api.judge.getTasks({
    source_ref: sourceRef || undefined,
    page: 1,
    size: 20,
  }), [sourceRef])

  /**
   * rejudgeTask 按原始快照触发后端重判。
   */
  const rejudgeTask = useCallback(async (taskId: string) => {
    setError(null)
    setMessage(null)
    try {
      await api.judge.rejudgeTask(taskId)
      setMessage('重判任务已提交。')
      resource.reload()
    } catch (actionError) {
      setError((actionError as ApiError).message || '重判任务提交失败，请稍后重试。')
    }
  }, [resource])

  const columns = useMemo<TableColumn<JudgeTask>[]>(() => [
    { key: 'task', title: '任务编号', dataIndex: 'task_id', priority: 'primary' },
    { key: 'submitter', title: '提交人', dataIndex: 'submitter_id' },
    { key: 'source', title: '来源引用', dataIndex: 'source_ref' },
    { key: 'status', title: '状态', render: (row) => <span className={styles.status}>{row.status}</span> },
    { key: 'score', title: '得分', render: (row) => row.result ? `${row.result.score}/${row.result.max_score}` : '待出分' },
    {
      key: 'actions',
      title: '操作',
      render: (row) => <Button variant="outline" size="sm" icon={<RotateCw size={14} />} onClick={() => rejudgeTask(row.task_id)}>重判</Button>,
    },
  ], [rejudgeTask])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Activity size={28} />实时判题与任务监控</h1>
          <p className={styles.subtitle}>查看判题任务状态，并按后端快照发起重判。</p>
        </div>
        <Button variant="outline" icon={<RefreshCw size={16} />} onClick={resource.reload}>刷新</Button>
      </div>
      {error && <div className={styles.error}>{error}</div>}
      {message && <Callout variant="success" title="操作成功">{message}</Callout>}
      <div className={styles.toolbar}>
        <Input placeholder="按来源引用筛选" value={sourceRef} onChange={(event) => setSourceRef(event.target.value)} />
      </div>
      {resource.status === 'error' && <ErrorState error={resource.error} onRetry={resource.reload} />}
      {resource.status === 'loading' && <LoadingState title="正在获取判题任务" />}
      {(resource.status === 'success' || resource.status === 'empty') && (
        <div className={styles.tableWrap}>
          <Table columns={columns} rows={rows} rowKey="task_id" emptyTitle="暂无判题任务" emptyDescription="当前筛选范围内没有判题任务。" ariaLabel="判题任务监控列表" />
        </div>
      )}
    </div>
  )
}

export default TeacherMonitoringPage
