// TeacherMonitoringPage 展示判题任务队列，并提供重判入口。

import React, { useCallback, useMemo, useState } from 'react'
import type { JudgeTask } from '@chaimir/api-client'
import type { TableColumn } from '@chaimir/ui'
import { Button, Callout, Checkbox, Input, Modal, Table, Textarea } from '@chaimir/ui'
import { Activity, Eye, RefreshCw, RotateCw, Save } from 'lucide-react'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource, useTicketedWebSocket } from '../../../../../hooks'
import styles from '../../judge.module.css'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { judgeTaskStatusLabel, sourceReferenceLabel } from '../../../../../utils'

const TeacherMonitoringPage: React.FC = () => {
  const [sourceRef, setSourceRef] = useState('')
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [selectedTask, setSelectedTask] = useState<JudgeTask | null>(null)
  const [score, setScore] = useState('0')
  const [maxScore, setMaxScore] = useState('100')
  const [passed, setPassed] = useState(false)
  const [comment, setComment] = useState('')
  const resource = useAsyncResource(() => api.judge.getTasks({
    source_ref: sourceRef || undefined,
    page: 1,
    size: 20,
  }), [sourceRef])
  const progressUrl = selectedTask ? api.judge.getProgressWsUrl(selectedTask.task_id) : null
  const progress = useTicketedWebSocket({
    url: progressUrl,
    onMessage: useCallback(() => {
      if (!selectedTask) return
      void api.judge.getTask(selectedTask.task_id).then(setSelectedTask).catch((taskError) => setError(userFacingErrorMessage(taskError, '判题进度刷新失败，请稍后重试。')))
    }, [selectedTask]),
  })

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
      setError(userFacingErrorMessage(actionError, '重判任务提交失败，请稍后重试。'))
    }
  }, [resource])

  /** openTask 读取判题任务完整结果并开启实时进度订阅。 */
  const openTask = useCallback(async (taskId: string) => {
    setError(null)
    try {
      setSelectedTask(await api.judge.getTask(taskId))
    } catch (taskError) {
      setError(userFacingErrorMessage(taskError, '判题任务读取失败，请稍后重试。'))
    }
  }, [])

  /** submitManualScore 提交教师人工评分并刷新任务详情。 */
  const submitManualScore = async () => {
    if (!selectedTask || !comment.trim()) return
    setError(null)
    try {
      setSelectedTask(await api.judge.manualScore(selectedTask.task_id, { score: Number(score), max_score: Number(maxScore), passed, comment: comment.trim() }))
      setMessage('人工评分已保存。')
      resource.reload()
    } catch (scoreError) {
      setError(userFacingErrorMessage(scoreError, '人工评分保存失败，请检查分数后重试。'))
    }
  }

  const columns = useMemo<TableColumn<JudgeTask>[]>(() => [
    { key: 'task', title: '任务编号', dataIndex: 'task_id', priority: 'primary' },
    { key: 'submitter', title: '提交人', dataIndex: 'submitter_id' },
    { key: 'source', title: '来源', render: (row) => sourceReferenceLabel(row.source_ref) },
    { key: 'status', title: '状态', render: (row) => <span className={styles.status}>{judgeTaskStatusLabel(row.status)}</span> },
    { key: 'score', title: '得分', render: (row) => row.result ? `${row.result.score}/${row.result.max_score}` : '待出分' },
    {
      key: 'actions',
      title: '操作',
      render: (row) => <div className={styles.actions}><Button variant="outline" size="sm" icon={<Eye size={14} />} onClick={() => void openTask(row.task_id)}>查看</Button><Button variant="ghost" size="sm" icon={<RotateCw size={14} />} onClick={() => rejudgeTask(row.task_id)}>重判</Button></div>,
    },
  ], [openTask, rejudgeTask])

  const rows = resource.data?.list || []

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <div>
          <h1 className={styles.title}><Activity size={28} />实时判题与任务监控</h1>
          <p className={styles.subtitle}>查看判题进度，并按原提交内容发起重判。</p>
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
      <Modal open={selectedTask !== null} title="判题任务详情" size="lg" onClose={() => setSelectedTask(null)}>
        {selectedTask && (
          <div className={styles.panel}>
            <span className={styles.status}>{progress.status === 'open' ? '实时进度已连接' : judgeTaskStatusLabel(selectedTask.status)}</span>
            <p>任务 {selectedTask.task_id}</p>
            <p>{selectedTask.result ? `当前得分 ${selectedTask.result.score}/${selectedTask.result.max_score}` : '正在等待判题结果。'}</p>
            <div className={styles.formGrid}>
              <label className={styles.field}>得分<Input fullWidth type="number" value={score} onChange={(event) => setScore(event.target.value)} /></label>
              <label className={styles.field}>满分<Input fullWidth type="number" value={maxScore} onChange={(event) => setMaxScore(event.target.value)} /></label>
              <Checkbox checked={passed} label="判定通过" onChange={(event) => setPassed(event.target.checked)} />
            </div>
            <label className={styles.field}>评分说明<Textarea fullWidth value={comment} onChange={(event) => setComment(event.target.value)} /></label>
            <Button icon={<Save size={14} />} onClick={() => void submitManualScore()}>保存人工评分</Button>
          </div>
        )}
      </Modal>
    </div>
  )
}

export default TeacherMonitoringPage
