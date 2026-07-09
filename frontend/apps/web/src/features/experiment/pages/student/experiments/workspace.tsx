// 学生实验工作台页：创建或读取后端实验实例，操作阶段、检查点和报告提交。

import React, { useMemo, useState } from 'react'
import type { ExperimentInstance } from '@chaimir/api-client'
import { EXPERIMENT_STAGE_STATUS } from '@chaimir/api-client'
import { Button, Input, Textarea } from '@chaimir/ui'
import { CheckCircle, Pause, Play, RefreshCw, Send, Trash2 } from 'lucide-react'
import { useParams, useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource } from '../../../../../hooks/useAsyncResource'
import styles from '../../experiment.module.css'

const ExperimentWorkspacePage: React.FC = () => {
  const { id } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const [activeStage, setActiveStage] = useState<number | null>(null)
  const [checkpointId, setCheckpointId] = useState('')
  const [codeStorageKey, setCodeStorageKey] = useState('')
  const [codeHash, setCodeHash] = useState('')
  const [reportRef, setReportRef] = useState('')
  const [message, setMessage] = useState('')
  const instanceId = searchParams.get('instanceId')

  const resource = useAsyncResource(
    async () => {
      if (instanceId) {
        return api.experiment.getInstance(instanceId)
      }
      if (!id) {
        throw new Error('缺少实验编号，无法创建实验实例。')
      }
      const instance = await api.experiment.createInstance(id, {})
      setSearchParams({ instanceId: instance.instance_id }, { replace: true })
      return instance
    },
    [id, instanceId],
    () => false
  )

  const instance = resource.data
  const firstEndpoint = useMemo(() => {
    return instance?.sandboxes.flatMap((sandbox) => sandbox.tools).find((tool) => tool.endpoint)?.endpoint
  }, [instance])

  const runInstanceAction = async (action: (id: string) => Promise<ExperimentInstance>, success: string) => {
    if (!instance) return
    setMessage('')
    try {
      await action(instance.instance_id)
      setMessage(success)
      resource.reload()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '操作没有完成，请稍后重试。')
    }
  }

  const activateStage = async (stage: number) => {
    if (!instance) return
    setMessage('')
    try {
      await api.experiment.activateStage(instance.instance_id, stage)
      setActiveStage(stage)
      setMessage('阶段已激活。')
      resource.reload()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '暂时无法激活该阶段。')
    }
  }

  const judgeCheckpoint = async () => {
    if (!instance || !checkpointId.trim()) return
    setMessage('')
    try {
      const result = await api.experiment.judgeCheckpoint(instance.instance_id, checkpointId.trim(), {
        code_storage_key: codeStorageKey.trim() || undefined,
        code_hash: codeHash.trim() || undefined,
      })
      setMessage(result.passed ? `检查点通过，获得 ${result.score} 分。` : '检查点未通过，请根据实验要求继续调整。')
      resource.reload()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '暂时无法提交检查点判分。')
    }
  }

  const submitReport = async () => {
    if (!instance || !reportRef.trim()) return
    setMessage('')
    try {
      await api.experiment.submitReport(instance.instance_id, { content_ref: reportRef.trim() })
      setMessage('实验报告已提交。')
      setReportRef('')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '暂时无法提交实验报告。')
    }
  }

  if (resource.status === 'loading') {
    return <LoadingState title="正在准备实验工作台" description="系统正在创建或读取你的实验实例。" />
  }

  if (resource.status === 'error') {
    return <ErrorState error={resource.error} onRetry={resource.reload} />
  }

  return (
    <div className={styles.workspace}>
      <aside className={styles.workspacePanel}>
        <h1 className={styles.workspaceTitle}>实验工作台</h1>
        <p className={styles.workspaceMeta}>实例 {instance?.instance_id}</p>
        <p className={styles.workspaceMeta}>当前得分 {instance?.score ?? 0}</p>
        {message && <p className={styles.workspaceMeta} role="status">{message}</p>}

        <div className={styles.darkCard}>
          <h2 className={styles.workspaceTitle}>阶段</h2>
          <ul className={styles.list}>
            {(instance?.stages ?? []).map((stage) => (
              <li key={stage.stage}>
                <button
                  className={`${styles.stageButton} ${activeStage === stage.stage || stage.status === EXPERIMENT_STAGE_STATUS.ACTIVE ? styles.stageButtonActive : ''}`}
                  disabled={stage.status === EXPERIMENT_STAGE_STATUS.LOCKED}
                  onClick={() => activateStage(stage.stage)}
                >
                  {stage.stage}. {stage.title}
                  <br />
                  <span>{stage.status === EXPERIMENT_STAGE_STATUS.LOCKED ? '未解锁' : stage.status === EXPERIMENT_STAGE_STATUS.ACTIVE ? '进行中' : '可进入'}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>

        <div className={styles.darkCard}>
          <h2 className={styles.workspaceTitle}>检查点判分</h2>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="checkpoint-id">检查点编号</label>
            <Input id="checkpoint-id" className={styles.darkInput} value={checkpointId} onChange={(event) => setCheckpointId(event.target.value)} fullWidth />
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="code-ref">代码存储引用</label>
            <Input id="code-ref" className={styles.darkInput} value={codeStorageKey} onChange={(event) => setCodeStorageKey(event.target.value)} fullWidth />
          </div>
          <div className={styles.field}>
            <label className={styles.label} htmlFor="code-hash">代码哈希</label>
            <Input id="code-hash" className={styles.darkInput} value={codeHash} onChange={(event) => setCodeHash(event.target.value)} fullWidth />
          </div>
          <Button size="sm" icon={<CheckCircle size={15} />} onClick={judgeCheckpoint}>提交判分</Button>
        </div>

        <div className={styles.darkCard}>
          <h2 className={styles.workspaceTitle}>实验报告</h2>
          <Textarea className={styles.darkInput} value={reportRef} onChange={(event) => setReportRef(event.target.value)} placeholder="填写实验报告内容引用" rows={3} fullWidth />
          <Button size="sm" icon={<Send size={15} />} onClick={submitReport}>提交报告</Button>
        </div>
      </aside>

      <main className={styles.embedStage}>
        <div className={styles.embedTopbar}>
          <div className={styles.toolbar}>
            <Button variant="on-dark" size="sm" icon={<RefreshCw size={15} />} onClick={resource.reload}>刷新实例</Button>
            <Button variant="on-dark" size="sm" icon={<Pause size={15} />} onClick={() => runInstanceAction(api.experiment.pauseInstance.bind(api.experiment), '实验实例已暂停。')}>暂停</Button>
            <Button variant="on-dark" size="sm" icon={<Play size={15} />} onClick={() => runInstanceAction(api.experiment.resumeInstance.bind(api.experiment), '实验实例已恢复。')}>恢复</Button>
            <Button variant="on-dark" size="sm" icon={<CheckCircle size={15} />} onClick={() => runInstanceAction(api.experiment.finishInstance.bind(api.experiment), '实验实例已完成。')}>完成</Button>
            <Button variant="danger" size="sm" icon={<Trash2 size={15} />} onClick={() => runInstanceAction(async (target) => { await api.experiment.recycleInstance(target); return instance as ExperimentInstance }, '实验资源已回收。')}>回收</Button>
          </div>
        </div>
        {firstEndpoint ? (
          <iframe className={styles.embedFrame} title="实验工具入口" src={firstEndpoint} />
        ) : (
          <div className={styles.panel}>
            <p className={styles.muted}>后端尚未返回可打开的工具入口，请刷新实例或稍后再试。</p>
          </div>
        )}
      </main>
    </div>
  )
}

export default ExperimentWorkspacePage
