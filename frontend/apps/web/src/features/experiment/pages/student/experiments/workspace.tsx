// 学生实验工作台页：复用共享 Sandbox IDE，并接入阶段、判分、报告和实例生命周期。

import React, { useCallback, useMemo, useState } from 'react'
import type { ExperimentInstance } from '@chaimir/api-client'
import { EXPERIMENT_STAGE_STATUS, ExperimentReportStatus } from '@chaimir/api-client'
import { Button, Callout, Input, Select, useConfirm, ResourceState } from '@chaimir/ui'
import { CheckCircle, FileUp, Pause, Play, Trash2 } from 'lucide-react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api } from '../../../../../app/api'
import { useAsyncResource, usePendingAction, useTicketedWebSocket } from '../../../../../hooks'
import { userFacingErrorMessage } from '../../../../../utils/userFacingError'
import { SandboxIdeWorkspace } from '../../../../sandbox/components/SandboxIdeWorkspace'
import styles from '../../experiment.module.css'

/** ExperimentWorkspacePage 以实验实例为业务层，把代码环境交给共享 Sandbox IDE。 */
const ExperimentWorkspacePage: React.FC = () => {
  const confirm = useConfirm()
  const navigate = useNavigate()
  const { id } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const [selectedSandboxId, setSelectedSandboxId] = useState('')
  const [activeStage, setActiveStage] = useState<number | null>(null)
  const [checkpointId, setCheckpointId] = useState('')
  const [codeStorageKey, setCodeStorageKey] = useState('')
  const [codeHash, setCodeHash] = useState('')
  const [reportFile, setReportFile] = useState<File | null>(null)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')
  const { pendingAction, runPendingAction } = usePendingAction()
  const instanceId = searchParams.get('instanceId')

  const resource = useAsyncResource(
    async () => {
      let instance: ExperimentInstance
      if (instanceId) {
        instance = await api.experiment.getInstance(instanceId)
      } else {
        if (!id) throw new Error('缺少实验编号，无法创建实验实例。')
        const experiment = await api.experiment.getPublishedExperiment(id)
        instance = await api.experiment.createInstance(id, { launch_grant: experiment.launch_grant })
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

  /** submitReport 把 Markdown 文件交给 M7 统一上传流程并刷新服务端提交状态。 */
  const submitReport = async () => {
    if (!instance || !reportFile) {
      setError('请选择 Markdown 格式的实验报告。')
      return
    }
    setMessage('')
    setError('')
    try {
      await api.experiment.submitReport(instance.instance_id, reportFile)
      setReportFile(null)
      setMessage(instance.report ? '实验报告已重新提交，原评分已清空。' : '实验报告已提交。')
      resource.reload()
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '实验报告提交失败，请检查文件后重试。'))
    }
  }

  /** recycleInstance 回收环境并返回实验列表，避免停留在已销毁工作台。 */
  const recycleInstance = async () => {
    if (!instance) return
    const confirmed = await confirm({ title: '回收实验资源', description: '回收后当前代码环境会被销毁，且无法继续操作这个实验实例。', confirmLabel: '确认回收' })
    if (!confirmed) return
    setError('')
    try {
      await api.experiment.recycleInstance(instance.instance_id)
      navigate('/student/experiments')
    } catch (actionError) {
      setError(userFacingErrorMessage(actionError, '实验资源回收失败，请稍后重试。'))
    }
  }

  if (resource.status === 'loading') return <ResourceState status="loading" title="正在准备实验工作台" description="系统正在创建或读取你的实验实例。" />
  if (resource.status === 'error') return <ResourceState status="error" error={resource.error} onRetry={resource.reload} />
  if (!instance) return <ResourceState status="loading" title="正在准备实验工作台" />

  const inspector = (
    <div className={styles.section}>
      {message && <Callout variant="success" title="操作完成">{message}</Callout>}
      {error && <Callout variant="danger" title="操作未完成">{error}</Callout>}
      <section className={styles.darkCard}>
        <h2 className={styles.workspaceTitle}>实验阶段</h2>
        <ul className={styles.list}>
          {(instance.stages || []).map((stage) => (
            <li key={stage.stage}>
              <button className={`${styles.stageButton} ${activeStage === stage.stage || stage.status === EXPERIMENT_STAGE_STATUS.ACTIVE ? styles.stageButtonActive : ''}`} disabled={Boolean(pendingAction) || stage.status === EXPERIMENT_STAGE_STATUS.LOCKED} aria-busy={pendingAction === `stage-${stage.stage}`} onClick={() => void runPendingAction(`stage-${stage.stage}`, () => activateStage(stage.stage))}>
                {stage.stage}. {stage.title}<br />
                <span>{stage.status === EXPERIMENT_STAGE_STATUS.LOCKED ? '未解锁' : stage.status === EXPERIMENT_STAGE_STATUS.ACTIVE ? '进行中' : '可进入'}</span>
              </button>
            </li>
          ))}
        </ul>
      </section>
      {instance.require_report && (
        <section className={styles.darkCard}>
          <h2 className={styles.workspaceTitle}>实验报告</h2>
          {instance.report && (
            <p>
              已提交：{instance.report.file_name}
              {instance.report.status === ExperimentReportStatus.GRADED ? `，得分 ${instance.report.manual_score} 分${instance.report.comment ? `，评语：${instance.report.comment}` : ''}` : '，等待教师批改'}
            </p>
          )}
          <input type="file" accept=".md,.markdown,text/markdown,text/plain" onChange={(event) => setReportFile(event.target.files?.[0] ?? null)} />
          <Button size="sm" icon={<FileUp size={15} />} loading={pendingAction === 'report'} disabled={Boolean(pendingAction) || !reportFile} onClick={() => void runPendingAction('report', submitReport)}>
            {instance.report ? '重新提交报告' : '提交报告'}
          </Button>
        </section>
      )}
      <section className={styles.darkCard}>
        <h2 className={styles.workspaceTitle}>检查点判分</h2>
        <Input value={checkpointId} onChange={(event) => setCheckpointId(event.target.value)} placeholder="检查点编号" fullWidth />
        <Button size="sm" icon={<CheckCircle size={15} />} loading={pendingAction === 'checkpoint'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('checkpoint', judgeCheckpoint)}>提交判分</Button>
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
      <Button variant="on-dark" size="sm" icon={<Pause size={15} />} loading={pendingAction === 'pause'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('pause', () => runInstanceAction(api.experiment.pauseInstance.bind(api.experiment), '实验实例已暂停。'))}>暂停</Button>
      <Button variant="on-dark" size="sm" icon={<Play size={15} />} loading={pendingAction === 'resume'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('resume', () => runInstanceAction(api.experiment.resumeInstance.bind(api.experiment), '实验实例已恢复。'))}>恢复</Button>
      <Button variant="on-dark" size="sm" icon={<CheckCircle size={15} />} loading={pendingAction === 'finish'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('finish', () => runInstanceAction(api.experiment.finishInstance.bind(api.experiment), '实验实例已完成。'))}>完成</Button>
      <Button variant="danger" size="sm" icon={<Trash2 size={15} />} loading={pendingAction === 'recycle'} disabled={Boolean(pendingAction)} onClick={() => void runPendingAction('recycle', recycleInstance)}>回收资源</Button>
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
