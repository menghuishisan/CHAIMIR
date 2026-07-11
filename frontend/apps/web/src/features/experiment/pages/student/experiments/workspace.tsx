// 学生实验工作台页：复用共享 Sandbox IDE，并接入阶段、判分、报告和实例生命周期。

import React, { useCallback, useMemo, useState } from 'react'
import type { ExperimentInstance } from '@chaimir/api-client'
import { EXPERIMENT_STAGE_STATUS } from '@chaimir/api-client'
import { Button, Callout, Input, Select, Textarea } from '@chaimir/ui'
import { CheckCircle, Pause, Play, Send, Trash2 } from 'lucide-react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { ErrorState, LoadingState } from '../../../../../components/ResourceState'
import { useAsyncResource, useTicketedWebSocket } from '../../../../../hooks'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { SandboxIdeWorkspace } from '../../../../sandbox/components/SandboxIdeWorkspace'
import styles from '../../experiment.module.css'

/** ExperimentWorkspacePage 以实验实例为业务层，把代码环境交给共享 Sandbox IDE。 */
const ExperimentWorkspacePage: React.FC = () => {
  const navigate = useNavigate()
  const { id } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const [selectedSandboxId, setSelectedSandboxId] = useState('')
  const [activeStage, setActiveStage] = useState<number | null>(null)
  const [checkpointId, setCheckpointId] = useState('')
  const [codeStorageKey, setCodeStorageKey] = useState('')
  const [codeHash, setCodeHash] = useState('')
  const [reportRef, setReportRef] = useState('')
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const instanceId = searchParams.get('instanceId')

  const resource = useAsyncResource(
    async () => {
      let instance: ExperimentInstance
      if (instanceId) {
        instance = await api.experiment.getInstance(instanceId)
      } else {
        if (!id) throw new Error('缺少实验编号，无法创建实验实例。')
        instance = await api.experiment.createInstance(id, {})
        setSearchParams({ instanceId: instance.instance_id }, { replace: true })
      }
      const progress = await api.experiment.getProgress(instance.instance_id)
      return { instance, progress }
    },
    [id, instanceId],
    () => false,
  )
  const instance = resource.data?.instance
  const progressSubscription = useMemo(
    () => resource.data?.progress.topic ? { action: 'subscribe', topics: [resource.data.progress.topic] } : undefined,
    [resource.data?.progress.topic],
  )
  const handleProgress = useCallback(() => resource.reload(), [resource])
  const realtime = useTicketedWebSocket({
    url: progressSubscription ? api.eventWebSocketUrl() : null,
    subscribeMessage: progressSubscription,
    onMessage: handleProgress,
  })
  const selectedSandbox = instance?.sandboxes.find((item) => item.sandbox_id === selectedSandboxId) || instance?.sandboxes[0]

  /** runInstanceAction 执行实例生命周期操作并刷新服务端权威状态。 */
  const runInstanceAction = async (action: (target: string) => Promise<ExperimentInstance>, success: string) => {
    if (!instance) return
    setMessage('')
    setError('')
    try {
      await action(instance.instance_id)
      setMessage(success)
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '操作没有完成，请稍后重试。'))
    }
  }

  /** activateStage 激活服务端阶段并等待编排结果回流。 */
  const activateStage = async (stage: number) => {
    if (!instance) return
    setMessage('')
    setError('')
    try {
      await api.experiment.activateStage(instance.instance_id, stage)
      setActiveStage(stage)
      setMessage('阶段已激活。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '暂时无法激活该阶段。'))
    }
  }

  /** judgeCheckpoint 使用共享 IDE 保存后返回的代码引用进行后端判分。 */
  const judgeCheckpoint = async () => {
    if (!instance || !checkpointId.trim()) return
    setMessage('')
    setError('')
    try {
      const result = await api.experiment.judgeCheckpoint(instance.instance_id, checkpointId.trim(), {
        code_storage_key: codeStorageKey || undefined,
        code_hash: codeHash || undefined,
      })
      setMessage(result.passed ? `检查点通过，获得 ${result.score} 分。` : '检查点未通过，请根据实验要求继续调整。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '暂时无法提交检查点判分。'))
    }
  }

  /** submitReport 提交服务端报告引用，提交完成后保留实例进度。 */
  const submitReport = async () => {
    if (!instance || !reportRef.trim()) return
    setMessage('')
    setError('')
    try {
      await api.experiment.submitReport(instance.instance_id, { content_ref: reportRef.trim() })
      setMessage('实验报告已提交。')
      setReportRef('')
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '暂时无法提交实验报告。'))
    }
  }

  /** recycleInstance 回收环境并返回实验列表，避免停留在已销毁工作台。 */
  const recycleInstance = async () => {
    if (!instance || !window.confirm('确定回收当前实验资源吗？回收后将无法继续操作这个实例。')) return
    setError('')
    try {
      await api.experiment.recycleInstance(instance.instance_id)
      navigate('/student/experiments')
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '实验资源回收失败，请稍后重试。'))
    }
  }

  if (resource.status === 'loading') return <LoadingState title="正在准备实验工作台" description="系统正在创建或读取你的实验实例。" />
  if (resource.status === 'error') return <ErrorState error={resource.error} onRetry={resource.reload} />
  if (!instance) return <LoadingState title="正在准备实验工作台" />

  const inspector = (
    <div className={styles.section}>
      {message && <Callout variant="success" title="操作完成">{message}</Callout>}
      {error && <Callout variant="danger" title="操作未完成">{error}</Callout>}
      <section className={styles.darkCard}>
        <h2 className={styles.workspaceTitle}>实验阶段</h2>
        <ul className={styles.list}>
          {(instance.stages || []).map((stage) => (
            <li key={stage.stage}>
              <button className={`${styles.stageButton} ${activeStage === stage.stage || stage.status === EXPERIMENT_STAGE_STATUS.ACTIVE ? styles.stageButtonActive : ''}`} disabled={stage.status === EXPERIMENT_STAGE_STATUS.LOCKED} onClick={() => void activateStage(stage.stage)}>
                {stage.stage}. {stage.title}<br />
                <span>{stage.status === EXPERIMENT_STAGE_STATUS.LOCKED ? '未解锁' : stage.status === EXPERIMENT_STAGE_STATUS.ACTIVE ? '进行中' : '可进入'}</span>
              </button>
            </li>
          ))}
        </ul>
      </section>
      <section className={styles.darkCard}>
        <h2 className={styles.workspaceTitle}>检查点判分</h2>
        <Input value={checkpointId} onChange={(event) => setCheckpointId(event.target.value)} placeholder="检查点编号" fullWidth />
        <Button size="sm" icon={<CheckCircle size={15} />} onClick={() => void judgeCheckpoint()}>提交判分</Button>
      </section>
      <section className={styles.darkCard}>
        <h2 className={styles.workspaceTitle}>实验报告</h2>
        <Textarea value={reportRef} onChange={(event) => setReportRef(event.target.value)} placeholder="填写已保存的实验报告地址" rows={3} fullWidth />
        <Button size="sm" icon={<Send size={15} />} onClick={() => void submitReport()}>提交报告</Button>
      </section>
      {instance.sims.map((sim) => (
        <Button key={sim.session_id} variant="outline" size="sm" icon={<Play size={15} />} onClick={() => navigate(`/student/simulations/${sim.package_code}?version=${encodeURIComponent(sim.version)}&sessionId=${encodeURIComponent(sim.session_id)}`)}>
          打开阶段 {sim.stage} 仿真
        </Button>
      ))}
    </div>
  )

  const controls = (
    <div className={styles.toolbar}>
      <Button variant="on-dark" size="sm" icon={<Pause size={15} />} onClick={() => void runInstanceAction(api.experiment.pauseInstance.bind(api.experiment), '实验实例已暂停。')}>暂停</Button>
      <Button variant="on-dark" size="sm" icon={<Play size={15} />} onClick={() => void runInstanceAction(api.experiment.resumeInstance.bind(api.experiment), '实验实例已恢复。')}>恢复</Button>
      <Button variant="on-dark" size="sm" icon={<CheckCircle size={15} />} onClick={() => void runInstanceAction(api.experiment.finishInstance.bind(api.experiment), '实验实例已完成。')}>完成</Button>
      <Button variant="danger" size="sm" icon={<Trash2 size={15} />} onClick={() => void recycleInstance()}>回收资源</Button>
    </div>
  )

  if (!selectedSandbox) {
    return (
      <div className={styles.page}>
        <header className={styles.header}><h1 className={styles.title}>实验工作台</h1></header>
        <Callout variant="info" title="当前阶段没有代码环境">可以进入阶段仿真，或激活包含代码环境的后续阶段。</Callout>
        {inspector}
        {controls}
      </div>
    )
  }

  return (
    <SandboxIdeWorkspace
      sandboxId={selectedSandbox.sandbox_id}
      title="实验工作台"
      subtitle={`当前得分 ${instance.score} · ${realtime.status === 'open' ? '实验进度已同步' : '正在同步实验进度'}`}
      inspector={inspector}
      controls={controls}
      actions={instance.sandboxes.length > 1 ? (
        <Select value={selectedSandbox.sandbox_id} onChange={setSelectedSandboxId} options={instance.sandboxes.map((sandbox) => ({ value: sandbox.sandbox_id, label: `阶段 ${sandbox.stage} · ${sandbox.runtime_code}` }))} />
      ) : undefined}
      onSaved={(result) => {
        setCodeStorageKey(result.code_storage_key)
        setCodeHash(result.code_hash)
        setMessage('代码已保存，可提交检查点判分。')
      }}
    />
  )
}

export default ExperimentWorkspacePage
