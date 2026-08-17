// 实验工作台(学生沉浸态,/student/experiments/:experimentId/workspace)。
//
// 三栏分工:左栏是当前该做什么(实验说明 + 本阶段内容),中间是干活的地方(沙箱代码环境
// 或仿真推演),右栏是区块孵化器(阶段链 + 检查点判分,创新①)。
//
// 深链与刷新都必须能回到同一个实验现场,所以实例编号走查询参数;缺失时按实验重新取回 ——
// 创建实例的接口对活跃实例幂等(对齐清单 §6.8),不会因为刷新多开一个环境。
//
// 边界:
//   出现哪些工作面由沙箱自己声明(SandboxIdeWorkspace 内按 capabilities 渲染),页面不猜;
//   进度经 M10 的统一业务 WS 订阅实例 topic(短时票据建连),收到推送就重读实例 ——
//   实例本身才是权威,推送只是「该刷新了」的信号,不用推送内容改本地状态。

import { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams, useSearchParams } from 'react-router'
import {
  FlaskConical,
  LoaderCircle,
  Pause,
  Play,
  RefreshCw,
  Send,
  SquareCheckBig,
  TriangleAlert,
} from 'lucide-react'
import {
  EXPERIMENT_STAGE_STATUS,
  ExperimentInstanceStatus,
  type ExperimentInstance,
  type SandboxRef,
  type SimSessionRef,
  type StudentExperiment,
} from '@chaimir/api-client'
import {
  Badge,
  Button,
  ChainProgress,
  Modal,
  ModalBody,
  ModalContent,
  ModalDescription,
  ModalFooter,
  ModalHeader,
  ModalTitle,
  Textarea,
  WorkbenchShell,
  WorkbenchTopbar,
  toast,
} from '@chaimir/ui'
import { api } from '../../../../app/api'
import { AppStatusScreen } from '../../../../components/AppStatusScreen'
import { SandboxIdeWorkspace } from '../../../sandbox/SandboxIdeWorkspace'
import { useTicketedWebSocket } from '../../../../hooks'
import { useImmersive } from '../../../../layouts/immersive/context'
import { experimentInstanceStatusLabel } from '../../../../utils/labels/experiment'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'
import { BlockIncubator, type WorkspaceCodeRef } from './block-incubator'

/** 查询参数名:实例编号随进入时写进地址,刷新与深链据此回到同一现场。 */
const INSTANCE_PARAM = 'instance'

/**
 * StudentExperimentWorkspacePage 装配实验工作台。
 */
export default function StudentExperimentWorkspacePage() {
  const { experimentId = '' } = useParams<{ experimentId: string }>()
  const [searchParams, setSearchParams] = useSearchParams()
  const { title, exit } = useImmersive()

  const [experiment, setExperiment] = useState<StudentExperiment>()
  const [instance, setInstance] = useState<ExperimentInstance>()
  const [fatalError, setFatalError] = useState<string>()

  const instanceId = searchParams.get(INSTANCE_PARAM) ?? ''

  /**
   * openWorkspace 取回实验与实例。
   * 没有实例编号时按实验重新取回:接口对活跃实例幂等,拿到的是同一个环境,
   * 取到后把编号写进地址,后续刷新就不再走这条路。
   */
  const openWorkspace = useCallback(async () => {
    setFatalError(undefined)
    try {
      const definition = await api.experiment.getPublishedExperiment(experimentId)
      setExperiment(definition)

      if (instanceId) {
        setInstance(await api.experiment.getInstance(instanceId))
        return
      }
      const created = await api.experiment.createInstance(experimentId, {
        group_id: definition.my_group_id,
      })
      setInstance(created)
      setSearchParams(
        (current) => {
          const next = new URLSearchParams(current)
          next.set(INSTANCE_PARAM, created.instance_id)
          return next
        },
        { replace: true }
      )
    } catch (error) {
      setFatalError(userFacingErrorMessage(error, '实验环境没能打开,请退出后重试。'))
    }
  }, [experimentId, instanceId, setSearchParams])

  useEffect(() => {
    void openWorkspace()
  }, [openWorkspace])

  /** reloadInstance 只重读实例:实验定义不会在会话中途变化。 */
  const reloadInstance = useCallback(async () => {
    if (!instanceId) return
    try {
      setInstance(await api.experiment.getInstance(instanceId))
    } catch (error) {
      // 重读失败不该把已经打开的工作台清空:提示后保留现场,由用户决定是否重试
      toast.error(userFacingErrorMessage(error, '实验状态刷新失败,稍后可以再试一次。'))
    }
  }, [instanceId])

  if (fatalError) {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        tone="danger"
        title="实验环境没能打开"
        description={fatalError}
        fullScreen={false}
        actions={
          <>
            <Button variant="on-dark" leftIcon={RefreshCw} onClick={() => void openWorkspace()}>
              重试
            </Button>
            <Button variant="on-dark" onClick={exit}>
              退出实验
            </Button>
          </>
        }
      />
    )
  }

  if (!experiment || !instance) {
    return (
      <AppStatusScreen
        icon={LoaderCircle}
        spinning
        title="正在准备实验环境"
        description="第一次进入需要启动环境,通常十几秒。"
        fullScreen={false}
      />
    )
  }

  return (
    <ExperimentWorkbench
      title={title}
      experiment={experiment}
      instance={instance}
      onExit={exit}
      onReload={reloadInstance}
    />
  )
}

interface ExperimentWorkbenchProps {
  title: string
  experiment: StudentExperiment
  instance: ExperimentInstance
  onExit: () => void
  onReload: () => void
}

/**
 * ExperimentWorkbench 持有工作台的交互状态与实例控制动作。
 */
function ExperimentWorkbench({
  title,
  experiment,
  instance,
  onExit,
  onReload,
}: ExperimentWorkbenchProps) {
  const [codeRef, setCodeRef] = useState<WorkspaceCodeRef>()
  const [activeSandboxId, setActiveSandboxId] = useState<string>()
  const [reportOpen, setReportOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string>()

  // 三处可选字段各自定住引用:直接写 `instance.stages ?? []` 会每次渲染换一个新数组,
  // 把下面按阶段过滤的 useMemo 变成每帧重算
  const stages = useMemo(() => instance.stages ?? [], [instance.stages])
  const results = useMemo(() => instance.checkpoints ?? [], [instance.checkpoints])
  const checkpoints = experiment.components.checkpoints

  // 当前阶段:后端把活跃阶段标成 active;没有分阶段的实验就只有一组资源
  const activeStage = useMemo(
    () => stages.find((stage) => stage.status === EXPERIMENT_STAGE_STATUS.ACTIVE)?.stage,
    [stages]
  )

  // 只呈现当前阶段的资源:老师按阶段编排环境,把全部阶段的沙箱一起摊出来会让人点错
  const sandboxes = useMemo(
    () => filterByStage(instance.sandboxes, activeStage),
    [activeStage, instance.sandboxes]
  )
  const sims = useMemo(
    () => filterByStage(instance.sims, activeStage),
    [activeStage, instance.sims]
  )

  const currentSandbox = useMemo(
    () => sandboxes.find((item) => item.sandbox_id === activeSandboxId) ?? sandboxes[0],
    [activeSandboxId, sandboxes]
  )

  // 进度订阅:topic 由实例给出,连的是 M10 统一业务 WS;推送只当刷新信号
  const [progressTopic, setProgressTopic] = useState<string>()
  useEffect(() => {
    let active = true
    void api.experiment
      .getProgress(instance.instance_id)
      .then((progress) => {
        if (active) setProgressTopic(progress.topic)
      })
      .catch((error: unknown) => {
        // 拿不到 topic 只影响自动刷新,不影响做实验:记日志,界面上用手动刷新兜住
        console.error('[experiment] 读取进度订阅信息失败', error)
      })
    return () => {
      active = false
    }
  }, [instance.instance_id])

  const eventUrl = useMemo(
    () => (progressTopic ? api.eventWebSocketUrl() : undefined),
    [progressTopic]
  )

  const socket = useTicketedWebSocket({
    url: eventUrl,
    onOpen: () => {
      if (progressTopic)
        socket.send(JSON.stringify({ action: 'subscribe', topics: [progressTopic] }))
    },
    onMessage: (data) => {
      // 推送内容只用来判断「是不是该刷新了」,状态一律回实例读
      if (data.includes('"type":"subscribed"')) return
      onReload()
    },
  })

  /** control 执行实例级动作(暂停/恢复/完成),完成后重读实例。 */
  const control = useCallback(
    async (action: 'pause' | 'resume' | 'finish') => {
      setBusy(true)
      setActionError(undefined)
      try {
        if (action === 'pause') await api.experiment.pauseInstance(instance.instance_id)
        if (action === 'resume') await api.experiment.resumeInstance(instance.instance_id)
        if (action === 'finish') await api.experiment.finishInstance(instance.instance_id)
        toast.success(
          action === 'pause'
            ? '实验已暂停,环境保留'
            : action === 'resume'
              ? '实验已恢复'
              : '实验已完成'
        )
        onReload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '这个操作没有完成,请稍后重试。'))
      } finally {
        setBusy(false)
      }
    },
    [instance.instance_id, onReload]
  )

  /** activateStage 激活已解锁阶段:阶段切换会带出该阶段的环境与仿真。 */
  const activateStage = useCallback(
    async (stage: number) => {
      setBusy(true)
      setActionError(undefined)
      try {
        await api.experiment.activateStage(instance.instance_id, stage)
        setActiveSandboxId(undefined)
        toast.success(`已进入第 ${stage} 阶段`)
        onReload()
      } catch (error) {
        setActionError(userFacingErrorMessage(error, '这个阶段还进不去,请稍后重试。'))
      } finally {
        setBusy(false)
      }
    },
    [instance.instance_id, onReload]
  )

  const paused = instance.status === ExperimentInstanceStatus.PAUSED
  const finished =
    instance.status === ExperimentInstanceStatus.FINISHED ||
    instance.status === ExperimentInstanceStatus.RECYCLED ||
    instance.status === ExperimentInstanceStatus.RELEASED

  const mintedCount = results.filter((result) => result.passed).length

  return (
    <>
      <WorkbenchShell
        workbench="experiment"
        topbar={
          <WorkbenchTopbar
            onExit={onExit}
            exitLabel="保存并退出"
            title={title}
            subtitle={experiment.name}
            progress={
              checkpoints.length > 0 ? (
                <ChainProgress
                  onDark
                  size="sm"
                  label="已铸区块"
                  total={checkpoints.length}
                  done={mintedCount}
                />
              ) : undefined
            }
            cta={
              experiment.require_report ? (
                <Button
                  variant="seal"
                  size="sm"
                  leftIcon={Send}
                  disabled={busy}
                  onClick={() => setReportOpen(true)}
                >
                  提交报告
                </Button>
              ) : (
                <Button
                  variant="seal"
                  size="sm"
                  leftIcon={SquareCheckBig}
                  loading={busy}
                  disabled={finished}
                  onClick={() => void control('finish')}
                >
                  完成实验
                </Button>
              )
            }
          />
        }
        left={
          <ExperimentBrief
            experiment={experiment}
            instance={instance}
            activeStage={activeStage}
            sandboxes={sandboxes}
            sims={sims}
            currentSandboxId={currentSandbox?.sandbox_id}
            onPickSandbox={setActiveSandboxId}
          />
        }
        leftLabel="实验说明"
        stage={
          currentSandbox ? (
            <SandboxIdeWorkspace
              key={currentSandbox.sandbox_id}
              sandboxId={currentSandbox.sandbox_id}
              onSaved={(result) =>
                setCodeRef({ codeStorageKey: result.code_storage_key, codeHash: result.code_hash })
              }
            />
          ) : (
            <NoEnvironmentStage sims={sims} />
          )
        }
        right={
          <BlockIncubator
            instanceId={instance.instance_id}
            checkpoints={checkpoints}
            results={results}
            stages={stages}
            codeRef={codeRef}
            onJudged={onReload}
            onActivateStage={(stage) => void activateStage(stage)}
          />
        }
        rightLabel="进度与判分"
        footer={
          <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-2">
            <div className="flex min-w-0 flex-col">
              <span className="flex flex-wrap items-center gap-2 text-xs text-on-dark-sub">
                <Badge onDark tone={paused ? 'warning' : finished ? 'neutral' : 'jade'}>
                  {experimentInstanceStatusLabel(instance.status)}
                </Badge>
                <span className="font-mono tabular-nums">当前得分 {instance.score}</span>
                {socket.status === 'open' ? <span>进度已连接</span> : null}
              </span>
              {actionError ? (
                <span className="mt-0.5 text-xs text-on-dark-danger">{actionError}</span>
              ) : null}
              {socket.error ? (
                <span className="mt-0.5 text-xs text-on-dark-sub">
                  {socket.error}进度不会自动刷新,可以手动刷新。
                </span>
              ) : null}
            </div>
            <div className="flex shrink-0 items-center gap-2">
              <Button variant="on-dark" size="sm" leftIcon={RefreshCw} onClick={onReload}>
                刷新状态
              </Button>
              {paused ? (
                <Button
                  variant="on-dark"
                  size="sm"
                  leftIcon={Play}
                  loading={busy}
                  onClick={() => void control('resume')}
                >
                  恢复实验
                </Button>
              ) : (
                <Button
                  variant="on-dark"
                  size="sm"
                  leftIcon={Pause}
                  loading={busy}
                  disabled={finished}
                  onClick={() => void control('pause')}
                >
                  暂停实验
                </Button>
              )}
            </div>
          </div>
        }
      />

      {reportOpen ? (
        <ReportModal
          instanceId={instance.instance_id}
          onClose={() => setReportOpen(false)}
          onSubmitted={() => {
            setReportOpen(false)
            onReload()
          }}
        />
      ) : null}
    </>
  )
}

interface ExperimentBriefProps {
  experiment: StudentExperiment
  instance: ExperimentInstance
  activeStage?: number
  sandboxes: SandboxRef[]
  sims: SimSessionRef[]
  currentSandboxId?: string
  onPickSandbox: (sandboxId: string) => void
}

/**
 * ExperimentBrief 渲染左栏:该做什么、这一阶段有哪些环境。
 */
function ExperimentBrief({
  experiment,
  instance,
  activeStage,
  sandboxes,
  sims,
  currentSandboxId,
  onPickSandbox,
}: ExperimentBriefProps) {
  return (
    <div className="flex h-full min-h-0 flex-col gap-4 overflow-y-auto p-4">
      <section className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <FlaskConical aria-hidden="true" className="size-4 text-accent" />
          <h2 className="text-sm font-medium text-on-dark">实验说明</h2>
        </div>
        <p className="whitespace-pre-wrap text-xs leading-relaxed text-on-dark-sub">
          {experiment.description || '老师没有写额外说明,按阶段任务推进即可。'}
        </p>
      </section>

      {activeStage !== undefined ? (
        <section className="flex flex-col gap-1">
          <h3 className="text-xs font-medium text-on-dark-sub">当前阶段</h3>
          <p className="text-sm text-on-dark">
            第 {activeStage} 阶段 ·{' '}
            {(instance.stages ?? []).find((stage) => stage.stage === activeStage)?.title ?? ''}
          </p>
        </section>
      ) : null}

      {sandboxes.length > 0 ? (
        <section className="flex flex-col gap-2">
          <h3 className="text-xs font-medium text-on-dark-sub">实验环境</h3>
          <ul className="flex flex-col gap-1">
            {sandboxes.map((sandbox) => (
              <li key={sandbox.sandbox_id}>
                <button
                  type="button"
                  onClick={() => onPickSandbox(sandbox.sandbox_id)}
                  className={
                    'hit-target relative flex w-full flex-col rounded-md border px-2 py-1.5 text-left focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2 ' +
                    (sandbox.sandbox_id === currentSandboxId
                      ? 'border-accent bg-dark-elevated'
                      : 'border-dark-line bg-dark-surface hover:bg-dark-elevated')
                  }
                >
                  <span className="truncate text-sm text-on-dark">{sandbox.component_id}</span>
                  <span className="truncate font-mono text-xs text-on-dark-sub">
                    {sandbox.runtime_code}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {sims.length > 0 ? (
        <section className="flex flex-col gap-2">
          <h3 className="text-xs font-medium text-on-dark-sub">仿真场景</h3>
          <ul className="flex flex-col gap-1">
            {sims.map((sim) => (
              <li
                key={sim.session_id}
                className="rounded-md border border-dark-line bg-dark-surface px-2 py-1.5"
              >
                <p className="truncate text-sm text-on-dark">{sim.component_id}</p>
                <p className="truncate font-mono text-xs text-on-dark-sub">
                  {sim.package_code} · {sim.version}
                </p>
              </li>
            ))}
          </ul>
          <p className="text-xs text-on-dark-faint">
            仿真场景在仿真工作台推演,推演结论回到这里做判分。
          </p>
        </section>
      ) : null}
    </div>
  )
}

/**
 * NoEnvironmentStage 说明这个阶段没有代码环境。
 * 有些实验只用仿真或只看结论,此时中间区不该是一片空白。
 */
function NoEnvironmentStage({ sims }: { sims: SimSessionRef[] }) {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-2 p-6 text-center">
      <p className="text-base text-on-dark">这个阶段没有代码环境</p>
      <p className="max-w-md text-sm text-on-dark-sub">
        {sims.length > 0
          ? '本阶段用仿真场景推演。去仿真实验室打开对应场景,推演完回到右栏判分。'
          : '本阶段不需要动手写代码,按右栏的检查点判分推进即可。'}
      </p>
    </div>
  )
}

interface ReportModalProps {
  instanceId: string
  onClose: () => void
  onSubmitted: () => void
}

/**
 * ReportModal 提交实验报告。
 * 报告正文以文本提交(后端存的是内容引用),提交后由老师批改 ——
 * 学生侧不做报告列表(那条接口在教师组),提交结果在实验详情看。
 */
function ReportModal({ instanceId, onClose, onSubmitted }: ReportModalProps) {
  const [content, setContent] = useState('')
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(async () => {
    if (content.trim() === '') {
      setFormError('请写下实验报告内容')
      return
    }
    setFormError(undefined)
    setWorking(true)
    try {
      await api.experiment.submitReport(instanceId, { content_ref: content.trim() })
      toast.success('报告已提交,等老师批改')
      onSubmitted()
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '提交没有成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [content, instanceId, onSubmitted])

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="xl">
        <ModalHeader>
          <ModalTitle>提交实验报告</ModalTitle>
          <ModalDescription>
            写清做法、遇到的问题与结论。提交后由老师批改,批改结果在实验详情里看。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-3">
          <Textarea
            aria-label="实验报告内容"
            value={content}
            rows={12}
            invalid={Boolean(formError)}
            onChange={(event) => setContent(event.target.value)}
          />
          {formError ? <p className="text-sm text-danger">{formError}</p> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            继续做实验
          </Button>
          <Button variant="seal" leftIcon={Send} loading={working} onClick={() => void submit()}>
            提交报告
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/**
 * filterByStage 取当前阶段的资源;实验没有分阶段(或阶段信息缺失)时全都给。
 * 后端把资源标了 stage,阶段化实验里跨阶段的资源不该同时出现在面板上。
 */
function filterByStage<T extends { stage: number }>(items: T[], activeStage?: number): T[] {
  if (activeStage === undefined) return items
  const matched = items.filter((item) => item.stage === activeStage)
  return matched.length > 0 ? matched : items.filter((item) => item.stage === 0)
}
