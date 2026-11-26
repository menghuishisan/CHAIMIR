// 仿真工作台按权威会话选择继续执行或只读回放。
// 内置包在 Worker 执行，扩展包只在服务端隔离容器执行；页面只渲染数据帧。

import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router'
import { Dices, LoaderCircle, Lock, Share2, TriangleAlert } from 'lucide-react'
import {
  Badge,
  Button,
  IconButton,
  toast,
  useReducedMotion,
} from '@chaimir/ui'
import { SimWorkerClient, isBuiltinSimulationCode } from '@chaimir/sim-sdk'
import type {
  InteractionDescriptor,
  JsonObject,
  RuntimeFrame,
  RuntimeSnapshot,
  SimInitParams,
  SimPackageDescriptor,
} from '@chaimir/sim-sdk'
import type { SimReplay } from '@chaimir/api-client'
import { api } from '../../../../app/api'
import { appConfig } from '../../../../app/config'
import { AppStatusScreen } from '../../../../components/AppStatusScreen'
import { useAsyncResource } from '../../../../hooks'
import { errorDiagnostics } from '../../../../utils/userFacingError'
import { useImmersive } from '../../../../layouts/immersive/context'
import { moveCommand, replayMoves } from '../../replayMoves'
import { createSimActionRecorder } from '../../actionRecorder'
import { useSimPlayback } from '../../playback'
import { SimPlaybackControls } from '../../components/SimPlaybackControls'
import { SimRuntimeWorkbench } from '../../components/SimRuntimeWorkbench'
import { SimShareModal } from '../../components/SimShareModal'
import { SimVersionMenu } from '../../components/SimVersionMenu'
import { SimReplayRuntime } from '../../components/SimReplayRuntime'
import { useIsolatedSimStream } from '../../isolatedStream'

/** 默认随机种子:固定值保证「同一个场景每次进来都一样」,换条件是显式动作。 */
const DEFAULT_SEED = 1

/**
 * StudentSimWorkspacePage 按场景来源选择执行位置,自身不接触仿真运行时。
 */
export default function StudentSimWorkspacePage() {
  const { packageCode = '' } = useParams<{ packageCode: string }>()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const { exit: defaultExit } = useImmersive()

  const version = searchParams.get('version') ?? ''
  const sessionId = searchParams.get('session') ?? undefined
  const returnExperiment = searchParams.get('return_experiment') ?? ''
  const returnInstance = searchParams.get('return_instance') ?? ''
  const exit = useCallback(() => {
    if (/^[1-9][0-9]*$/.test(returnExperiment) && /^[1-9][0-9]*$/.test(returnInstance)) {
      navigate(`/student/experiments/${returnExperiment}/workspace?instance=${returnInstance}`)
      return
    }
    defaultExit()
  }, [defaultExit, navigate, returnExperiment, returnInstance])

  if (!sessionId && !isBuiltinSimulationCode(packageCode)) {
    return (
      <AppStatusScreen
        icon={Lock}
        title="这个场景要从课程里进入"
        description="它是本校自建的仿真场景,运行在学校的服务器上,需要由课程实验为你准备好推演环境。请在课程的实验任务里打开它。"
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={exit}>
            返回仿真实验室
          </Button>
        }
      />
    )
  }

  return <SimSession packageCode={packageCode} version={version} sessionId={sessionId} onExit={exit} />
}

interface SimSessionProps {
  packageCode: string
  version: string
  sessionId?: string
  onExit: () => void
}

// 包版本、种子和已有动作由会话决定，不能用地址参数覆盖。
function SimSession({ packageCode, version, sessionId, onExit }: SimSessionProps) {
  const { title } = useImmersive()
  const replay = useAsyncResource(
    () => (sessionId ? api.sim.getReplay(sessionId) : Promise.resolve(undefined)),
    [sessionId],
    () => false,
  )

  if (sessionId && replay.status === 'loading') {
    return (
      <AppStatusScreen icon={LoaderCircle} spinning title="正在恢复上次的推演" fullScreen={false} />
    )
  }

  if (sessionId && replay.status === 'error') {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        tone="danger"
        title="上次的推演没能恢复"
        description={replay.error?.message}
        traceId={replay.error?.traceId}
        fullScreen={false}
        actions={
          <>
            <Button variant="on-dark" onClick={replay.reload}>
              重试
            </Button>
            <Button variant="on-dark" onClick={onExit}>
              返回仿真实验室
            </Button>
          </>
        }
      />
    )
  }

  const code = replay.data?.package_code ?? packageCode
  const selectedVersion = replay.data?.version ?? version
  if (sessionId && !isBuiltinSimulationCode(code)) {
    return <IsolatedRuntime key={sessionId} version={selectedVersion} sessionId={sessionId} onExit={onExit} />
  }
  if (replay.data?.read_only) {
    return <SimReplayRuntime key={sessionId} replay={replay.data} title={title} onExit={onExit} />
  }
  return (
    <SimRuntime
      key={`${code}:${sessionId ?? 'local'}`}
      packageCode={code}
      version={selectedVersion}
      sessionId={sessionId}
      restore={replay.data ?? undefined}
      onExit={onExit}
    />
  )
}

interface SimRuntimeProps {
  packageCode: string
  version: string
  sessionId?: string
  /** 带会话时的服务端记录:据此恢复现场并续上上报序号 */
  restore?: SimReplay
  onExit: () => void
}

/**
 * SimRuntime 持有 Worker 生命周期、推演控制与操作上报。
 * 推演进度不另存一份:Worker 内的事件日志才是权威(snapshot.events),
 * 页面再记一个游标只会与后退、重播产生漂移。
 */
function SimRuntime({ packageCode, version, sessionId, restore, onExit }: SimRuntimeProps) {
  const { title } = useImmersive()
  const reducedMotion = useReducedMotion()

  // 带会话时随记录走(seed 决定这条过程),否则用默认随机条件
  const [seed, setSeed] = useState(restore ? restore.seed : DEFAULT_SEED)
  const [client, setClient] = useState<SimWorkerClient>()
  const [descriptor, setDescriptor] = useState<SimPackageDescriptor>()
  const [snapshot, setSnapshot] = useState<RuntimeSnapshot>()
  const [error, setError] = useState<string>()
  const [shareOpen, setShareOpen] = useState(false)
  // 回退或重播后就偏离了服务端记录(那是只追加的序列,表达不了撤回),此后不再上报
  const [diverged, setDiverged] = useState(false)
  const [reportFailed, setReportFailed] = useState(false)
  const [savedActionCount, setSavedActionCount] = useState(restore?.actions.length ?? 0)
  const [recordAction] = useState(
    () =>
      createSimActionRecorder(
        restore?.actions.length ?? 0,
        (action) => api.sim.reportAction(sessionId ?? '', action),
        (action) => setSavedActionCount(action.seq),
        (actionError, action) => {
          setReportFailed(true)
          console.error('[sim] 操作上报失败', {
            sessionId,
            eventType: action.event_type,
            error: errorDiagnostics(actionError),
          })
          toast.error('这一步没能记录到服务端，后续操作不会加入这条回放。')
        },
      ),
  )

  // 换过随机条件:那已经不是这条记录的过程了,既不恢复也不上报。
  // 它与 diverged 分开:换条件要重新装配 Worker(过程从头不同),
  // 而回退只是在同一条过程上往回走,绝不能因此重建 Worker —— 那会把现场清空。
  const seedChanged = restore !== undefined && seed !== restore.seed
  const reportBlocked = diverged || seedChanged || reportFailed

  const restoreMoves = useMemo(
    () => (restore && !seedChanged ? replayMoves(restore.actions) : []),
    [restore, seedChanged],
  )

  /** sendCommand 发出一条 Worker 命令并把失败原因记为页面错误态。 */
  const sendCommand = useCallback((command: Promise<void>) => {
    void command.catch((commandError: unknown) => {
      console.error('仿真命令执行失败', { operation: 'sim.student.command', error: errorDiagnostics(commandError) })
      setError('仿真运行失败,请重新进入后重试。')
    })
  }, [])

  // Worker 与「这个场景 + 这个种子」同生共死:换种子即重新装配,得到一条新的确定性过程
  useEffect(() => {
    let active = true
    const fail = (cause: unknown) => {
      if (!active) return
      console.error('仿真运行时报告错误', { operation: 'sim.student.runtime', error: errorDiagnostics(cause) })
      setError('仿真运行失败,请重新进入后重试。')
    }
    const runtime = new SimWorkerClient({
      builtinCode: packageCode,
      initParams: restore && !seedChanged ? (restore.init_params as SimInitParams) : {},
      seed,
      commandTimeoutMs: appConfig.simWorkerCommandTimeoutMs,
      onReady: (readyDescriptor, readySnapshot) => {
        if (!active) return
        setDescriptor(readyDescriptor)
        setSnapshot(readySnapshot)
      },
      onSnapshot: (nextSnapshot) => {
        if (!active) return
        setSnapshot(nextSnapshot)
      },
      onError: fail,
    })
    // 恢复现场:按记录逐条重放到上次离开的位置。逐条串行是必须的 ——
    // Worker 只按命令推进时刻,并发发出会打乱先后顺序
    const boot = async () => {
      await runtime.init()
      for (const move of restoreMoves) {
        if (!active) return
        await moveCommand(runtime, move)
      }
      if (active) setClient(runtime)
    }
    void boot().catch(fail)

    return () => {
      active = false
      runtime.destroy()
    }
  }, [packageCode, restore, restoreMoves, seed, seedChanged])

  // 自动推进一步:交给共享播放控制按当前速度排下一步(见 features/sim/playback.ts)
  const advance = useCallback(() => {
    if (!client) return
    sendCommand(client.step())
  }, [client, sendCommand])

  // 减弱动效偏好下不自动推进(规范 §7.3:冻结并保留当前态文字),单步仍然可用
  const playback = useSimPlayback({
    advance,
    canAdvance: Boolean(client) && !error && snapshot?.interactionAvailability.advance !== false,
    cue: snapshot,
    autoPlay: !reducedMotion,
  })

  // 记录 Worker 确认的事件；回退或更换随机条件后已偏离原回放，不再追加。
  const interact = useCallback(
    (interaction: InteractionDescriptor, payload: JsonObject, target?: string) => {
      if (!client || !snapshot) return
      playback.stop()
      const shouldRecord = Boolean(sessionId) && !reportBlocked
      sendCommand(client.inject(interaction.emits, payload, target).then((event) => {
        if (shouldRecord) {
          recordAction({ at_tick: event.atTick, event_type: event.type, payload: event.payload })
        }
      }))
    },
    [client, playback, recordAction, reportBlocked, sendCommand, sessionId, snapshot],
  )

  /** selectElement 选中舞台上的一个元素:它也是一次用户操作,同样进 Worker 并上报。 */
  const selectElement = useCallback(
    (elementId: string) => {
      const interaction = descriptor?.interactions.find((item) => item.kind === 'select-element')
      if (!interaction) return
      interact(interaction, {}, elementId)
    },
    [descriptor, interact],
  )

  /** stepOnce 手动走一步:接管节奏,停下自动推进。 */
  const stepOnce = useCallback(() => {
    playback.stop()
    advance()
  }, [advance, playback])

  /**
   * stepBack 回退一步:Worker 从初始状态重放到上一条事件,状态可复现,不是就地反算。
   * 带会话时回退即脱离服务端记录:那份记录是只追加的操作序列,表达不了「撤回一步」,
   * 之后继续上报会与已有条目撞号。故这里显式标记脱离,不再往记录里追加。
   */
  const stepBack = useCallback(() => {
    if (!client) return
    playback.stop()
    if (sessionId) setDiverged(true)
    sendCommand(client.back())
  }, [client, playback, sendCommand, sessionId])

  /** restart 回到初始状态:同回退,带会话时一并脱离记录(不重置序号,记录不可改写)。 */
  const restart = useCallback(() => {
    if (!client) return
    playback.stop()
    if (sessionId) setDiverged(true)
    sendCommand(client.reset())
  }, [client, playback, sendCommand, sessionId])

  // 运行环境装配失败时没有可看的画面,直接给出说明与唯一出口
  if (error && !client) {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        tone="danger"
        title="这个场景没能跑起来"
        description={error}
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={onExit}>
            返回仿真实验室
          </Button>
        }
      />
    )
  }

  if (!client || !descriptor || !snapshot) {
    return (
      <AppStatusScreen
        icon={LoaderCircle}
        spinning
        title={restoreMoves.length ? '正在恢复上次的推演' : '正在装配仿真场景'}
        fullScreen={false}
      />
    )
  }

  return (
    <>
      <SimRuntimeWorkbench
        title={title}
        version={version}
        descriptor={descriptor}
        snapshot={snapshot}
        onExit={onExit}
        action={sessionId ? <Button variant="primary" size="sm" leftIcon={Share2} onClick={() => setShareOpen(true)}>分享推演</Button> : undefined}
        titleExtra={
          <>
            <IconButton
              variant="on-dark"
              size="sm"
              icon={Dices}
              aria-label="换一组条件"
              title="换一组条件"
              disabled={Boolean(error)}
              onClick={() => {
                setClient(undefined)
                setSnapshot(undefined)
                setSeed((current) => current + 1)
              }}
            />
            {sessionId ? null : <SimVersionMenu packageCode={packageCode} version={version} />}
          </>
        }
        onSelectElement={selectElement}
        onInteract={interact}
        footer={
          <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-2">
            <div className="flex min-w-0 flex-col">
              <span className="flex flex-wrap items-center gap-2 font-mono text-xs tabular-nums text-on-dark-sub">
                <span>推演时刻 {snapshot.tick}</span>
                <span>随机条件 {seed}</span>
                {sessionId ? (
                  reportBlocked ? (
                    <Badge onDark tone="warning">
                      {reportFailed ? '记录已中断' : '已脱离记录'}
                    </Badge>
                  ) : (
                    <Badge onDark tone="jade">已记录 {savedActionCount} 步</Badge>
                  )
                ) : (
                  <Badge onDark tone="neutral">本机推演</Badge>
                )}
              </span>
              {error ? <span className="mt-0.5 text-xs text-on-dark-danger">{error}</span> : null}
              {sessionId && reportBlocked ? (
                <span className="mt-0.5 text-xs text-on-dark-sub">
                  {reportFailed
                    ? '服务端记录中断了，后续操作只在本机推演。刷新页面可从最后保存的记录恢复。'
                    : '回退、重播或换条件后不会再追加到服务端；分享内容仍是原来的操作记录。'}
                </span>
              ) : null}
            </div>
            <SimPlaybackControls
              playing={playback.playing}
              onToggle={playback.toggle}
              onStep={stepOnce}
              onStepBack={stepBack}
              onRestart={restart}
              multiplier={playback.multiplier}
              onMultiplierChange={playback.setMultiplier}
              disabled={Boolean(error)}
              atEnd={!error && snapshot.interactionAvailability.advance === false}
              atStart={snapshot.events.length === 0}
              playLabel="自动推进"
            />
          </div>
        }
      />

      {shareOpen && sessionId ? (
        <SimShareModal sessionId={sessionId} onClose={() => setShareOpen(false)} />
      ) : null}
    </>
  )
}

interface IsolatedRuntimeProps {
  version: string
  sessionId: string
  onExit: () => void
}

/**
 * IsolatedRuntime 渲染在服务端隔离容器内运行的本校自建场景。
 *
 * 与本机推演只有三处不同:过程由服务端持有(刷新后由服务端按已登记操作重放回来)、
 * 操作由服务端在执行成功后自行登记(前端不重复上报)、随机条件由会话决定(这里不能换)。
 * 舞台、面板与播放控制完全共用 —— 页面不为「跑在哪」写第二套渲染。
 */
function IsolatedRuntime({ version, sessionId, onExit }: IsolatedRuntimeProps) {
  const { title } = useImmersive()
  const reducedMotion = useReducedMotion()
  const [shareOpen, setShareOpen] = useState(false)

  const stream = useIsolatedSimStream(sessionId)
  const { descriptor, snapshot } = stream

  // 减弱动效偏好下不自动推进(规范 §7.3),单步仍然可用
  const playback = useSimPlayback({
    advance: stream.step,
    canAdvance: Boolean(descriptor && snapshot) && stream.status === 'open' && !stream.error && snapshot?.interactionAvailability.advance !== false,
    cue: snapshot,
    autoPlay: !reducedMotion,
  })

  const interact = useCallback(
    (interaction: InteractionDescriptor, payload: JsonObject, target?: string) => {
      playback.stop()
      stream.inject(interaction.emits, payload, target)
    },
    [playback, stream],
  )

  const selectElement = useCallback(
    (elementId: string) => {
      const interaction = descriptor?.interactions.find((item) => item.kind === 'select-element')
      if (!interaction) return
      interact(interaction, {}, elementId)
    },
    [descriptor, interact],
  )

  const stepOnce = useCallback(() => {
    playback.stop()
    stream.step()
  }, [playback, stream])

  const stepBack = useCallback(() => {
    playback.stop()
    stream.back()
  }, [playback, stream])

  const restart = useCallback(() => {
    playback.stop()
    stream.restart()
  }, [playback, stream])

  // 连接建立不起来时没有可看的画面,给出说明与两个出口
  if (stream.error && !snapshot) {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        tone="danger"
        title="没能连上服务器上的推演环境"
        description={stream.error}
        fullScreen={false}
        actions={
          <>
            <Button variant="on-dark" onClick={stream.reconnect}>
              重试
            </Button>
            <Button variant="on-dark" onClick={onExit}>
              返回仿真实验室
            </Button>
          </>
        }
      />
    )
  }

  if (!descriptor || !snapshot) {
    return (
      <AppStatusScreen
        icon={LoaderCircle}
        spinning
        title="正在准备服务器上的推演环境"
        description="学校自建场景在服务器的隔离环境里运行,首次进入需要等待环境就绪。"
        fullScreen={false}
      />
    )
  }

  return (
    <>
      <SimRuntimeWorkbench
        title={title}
        version={version}
        descriptor={descriptor}
        snapshot={snapshot}
        onExit={onExit}
        action={stream.readOnly ? undefined : <Button variant="primary" size="sm" leftIcon={Share2} onClick={() => setShareOpen(true)}>分享推演</Button>}
        titleExtra={stream.readOnly ? <Badge onDark tone="neutral">只读回放</Badge> : undefined}
        onSelectElement={stream.readOnly || stream.status !== 'open' || stream.error ? undefined : selectElement}
        onInteract={stream.readOnly || stream.status !== 'open' || stream.error ? undefined : interact}
        footer={<IsolatedFooter stream={stream} playback={playback} snapshot={snapshot} onStep={stepOnce} onStepBack={stepBack} onRestart={restart} />}
      />

      {shareOpen && !stream.readOnly ? (
        <SimShareModal sessionId={sessionId} onClose={() => setShareOpen(false)} />
      ) : null}
    </>
  )
}

interface IsolatedFooterProps {
  stream: ReturnType<typeof useIsolatedSimStream>
  playback: ReturnType<typeof useSimPlayback>
  snapshot: RuntimeFrame
  onStep: () => void
  onStepBack: () => void
  onRestart: () => void
}

/**
 * IsolatedFooter 渲染服务端推演的状态条与播放控制。
 * 状态条不写「已记录 N 步」:操作由服务端在执行成功后登记,前端不持有那个计数,
 * 也就不该猜一个数字给学生看。
 */
function IsolatedFooter({
  stream,
  playback,
  snapshot,
  onStep,
  onStepBack,
  onRestart,
}: IsolatedFooterProps) {
  const disabled = stream.status !== 'open' || Boolean(stream.error)
  return (
    <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-2">
      <div className="flex min-w-0 flex-col">
        <span className="flex flex-wrap items-center gap-2 font-mono text-xs tabular-nums text-on-dark-sub">
          <span>推演时刻 {snapshot.tick}</span>
          <Badge onDark tone={stream.status === 'open' ? 'jade' : 'warning'}>
            {stream.status === 'open' ? (stream.readOnly ? '只读回放' : '正在推演') : '连接已断开'}
          </Badge>
        </span>
        {stream.error ? (
          <span className="mt-0.5 flex items-center gap-2 text-xs text-on-dark-danger">
            {stream.error}
            <Button variant="on-dark" size="sm" onClick={stream.reconnect}>重新连接</Button>
          </span>
        ) : null}
      </div>
      <SimPlaybackControls
        playing={playback.playing}
        onToggle={playback.toggle}
        onStep={onStep}
        onStepBack={onStepBack}
        onRestart={onRestart}
        multiplier={playback.multiplier}
        onMultiplierChange={playback.setMultiplier}
        disabled={disabled}
        atEnd={!stream.error && snapshot.interactionAvailability.advance === false}
        atStart={stream.eventCount === 0}
        playLabel={stream.readOnly ? '播放' : '自动推进'}
      />
    </div>
  )
}
