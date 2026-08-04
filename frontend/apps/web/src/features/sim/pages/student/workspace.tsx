// 仿真工作台(学生沉浸态,/student/simulations/:packageCode/workspace)。
//
// 场景在隔离 Worker 里确定性运行:同一个 seed 与同一串操作必然得到同一过程 ——
// 这是仿真能当教学材料用的前提,也是回放能成立的原因。主线程只收纯数据快照,不执行场景代码。
//
// 两种模式:
//   自由推演(默认):学生自己从仿真实验室进来,没有会话。会话由 M7 实验编排产生
//     (POST /sim/sessions 在 internal 组,对齐清单 §6.6),所以此时不上报动作、不分享。
//   带会话推演:地址上带 session 时把每一次「用户操作」按连续序号上报服务端,
//     据此可以出分享码、也可以读回服务端记录。自动推进的 tick 不上报(它由 seed 决定,可复算)。
//
// 运行边界与公开回放一致:只运行平台内置场景。扩展场景的运行包契约未定,
// 不做降级渲染,直接说明清楚。

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import {
  Dices,
  Link2,
  LoaderCircle,
  Lock,
  Pause,
  Play,
  RotateCcw,
  Share2,
  StepBack,
  StepForward,
  TriangleAlert,
} from 'lucide-react'
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
  TeachingFrameAside,
  TeachingFrameBrief,
  TeachingFrameStage,
  WorkbenchShell,
  WorkbenchTopbar,
  frameHasAside,
  toast,
} from '@chaimir/ui'
import { SimWorkerClient, isBuiltinSimulationCode } from '@chaimir/sim-sdk'
import type {
  InteractionDescriptor,
  JsonObject,
  JsonValue,
  RuntimeSnapshot,
  SimInitParams,
  SimPackageDescriptor,
} from '@chaimir/sim-sdk'
import type { SimReplay } from '@chaimir/api-client'
import { api } from '../../../../app/api'
import { appConfig } from '../../../../app/config'
import { AppStatusScreen } from '../../../../components/AppStatusScreen'
import { useAsyncResource } from '../../../../hooks'
import { useImmersive } from '../../../../layouts/immersive/context'
import { moveCommand, replayMoves } from '../../replayMoves'
import { userFacingErrorMessage } from '../../../../utils/userFacingError'

/** 默认随机种子:固定值保证「同一个场景每次进来都一样」,换条件是显式动作。 */
const DEFAULT_SEED = 1

/** 交互标签的用户向说明:扰动与攻击类操作会破坏系统,提前说清。 */
const TAG_LABELS: Record<'normal' | 'perturb' | 'attack', string> = {
  normal: '常规操作',
  perturb: '扰动',
  attack: '攻击',
}

/**
 * StudentSimWorkspacePage 判定可运行边界,自身不接触仿真运行时。
 */
export default function StudentSimWorkspacePage() {
  const { packageCode = '' } = useParams<{ packageCode: string }>()
  const [searchParams] = useSearchParams()
  const { exit } = useImmersive()

  const version = searchParams.get('version') ?? ''
  const sessionId = searchParams.get('session') ?? undefined

  // 运行边界:只运行平台内置场景。扩展场景的运行包要按学校资产授权取回,契约未定,不降级
  if (!isBuiltinSimulationCode(packageCode)) {
    return (
      <AppStatusScreen
        icon={Lock}
        title="这个场景还不能在浏览器里推演"
        description="它是本校自建的仿真场景,运行包的取用方式还没有开放。请联系老师在课堂环境中演示。"
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={exit}>
            返回仿真实验室
          </Button>
        }
      />
    )
  }

  return <SimSession packageCode={packageCode} version={version} sessionId={sessionId} />
}

interface SimSessionProps {
  packageCode: string
  version: string
  sessionId?: string
}

/**
 * SimSession 决定这次推演从哪里开始。
 *
 * 带会话时先取回服务端记录(FE-9:刷新、换设备都不能丢已经做过的操作):
 * 记录里带原始 seed、初始参数与全部动作,据此把现场恢复到上次离开的样子,
 * 上报序号也从已有条数续上 —— 从 1 重开会与已有记录撞号,后端会拒。
 * 没有会话就是一次全新的本机推演,用默认随机条件开始。
 */
function SimSession({ packageCode, version, sessionId }: SimSessionProps) {
  const { exit } = useImmersive()

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
        title="上次的推演没能恢复"
        description={replay.error?.message}
        traceId={replay.error?.traceId}
        fullScreen={false}
        actions={
          <>
            <Button variant="on-dark" onClick={replay.reload}>
              重试
            </Button>
            <Button variant="on-dark" onClick={exit}>
              返回仿真实验室
            </Button>
          </>
        }
      />
    )
  }

  return (
    <SimRuntime
      packageCode={packageCode}
      version={version}
      sessionId={sessionId}
      restore={replay.data ?? undefined}
    />
  )
}

interface SimRuntimeProps {
  packageCode: string
  version: string
  sessionId?: string
  /** 带会话时的服务端记录:据此恢复现场并续上上报序号 */
  restore?: SimReplay
}

/**
 * SimRuntime 持有 Worker 生命周期、推演控制与操作上报。
 * 推演进度不另存一份:Worker 内的事件日志才是权威(snapshot.events),
 * 页面再记一个游标只会与后退、重播产生漂移。
 */
function SimRuntime({ packageCode, version, sessionId, restore }: SimRuntimeProps) {
  const { title, exit } = useImmersive()

  // 带会话时随记录走(seed 决定这条过程),否则用默认随机条件
  const [seed, setSeed] = useState(restore ? restore.seed : DEFAULT_SEED)
  const [client, setClient] = useState<SimWorkerClient>()
  const [descriptor, setDescriptor] = useState<SimPackageDescriptor>()
  const [snapshot, setSnapshot] = useState<RuntimeSnapshot>()
  const [error, setError] = useState<string>()
  const [playing, setPlaying] = useState(false)
  const [shareOpen, setShareOpen] = useState(false)
  const [restoring, setRestoring] = useState(false)
  // 回退或重播后就偏离了服务端记录(那是只追加的序列,表达不了撤回),此后不再上报
  const [diverged, setDiverged] = useState(false)

  // 上报序号:从服务端已有条数续上,只数用户操作(自动 tick 不上报,它由 seed 决定可复算)
  const reportSeq = useRef(restore ? restore.actions.length : 0)

  // 换过随机条件:那已经不是这条记录的过程了,既不恢复也不上报。
  // 它与 diverged 分开:换条件要重新装配 Worker(过程从头不同),
  // 而回退只是在同一条过程上往回走,绝不能因此重建 Worker —— 那会把现场清空。
  const seedChanged = restore !== undefined && seed !== restore.seed
  const reportBlocked = diverged || seedChanged

  const restoreMoves = useMemo(
    () => (restore && !seedChanged ? replayMoves(restore.actions) : []),
    [restore, seedChanged],
  )

  /** sendCommand 发出一条 Worker 命令并把失败原因记为页面错误态。 */
  const sendCommand = useCallback((command: Promise<void>) => {
    void command.catch((commandError: Error) => setError(commandError.message))
  }, [])

  // Worker 与「这个场景 + 这个种子」同生共死:换种子即重新装配,得到一条新的确定性过程
  useEffect(() => {
    let active = true
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
      onError: (message) => {
        if (!active) return
        setError(message)
      },
    })
    setClient(runtime)

    // 恢复现场:按记录逐条重放到上次离开的位置。逐条串行是必须的 ——
    // Worker 只按命令推进时刻,并发发出会打乱先后顺序
    const boot = async () => {
      await runtime.init()
      if (restoreMoves.length === 0) return
      setRestoring(true)
      try {
        for (const move of restoreMoves) {
          if (!active) return
          await moveCommand(runtime, move)
        }
      } finally {
        if (active) setRestoring(false)
      }
    }
    sendCommand(boot())

    return () => {
      active = false
      runtime.destroy()
    }
  }, [packageCode, restore, restoreMoves, seed, seedChanged, sendCommand])

  // 自动推进:每一步都在上一步的快照回来之后再排,命令不会在慢帧上堆积
  useEffect(() => {
    if (!playing || !client || error) return
    const timer = setTimeout(() => sendCommand(client.step()), simStepIntervalMs())
    return () => clearTimeout(timer)
  }, [client, error, playing, sendCommand, snapshot])

  /**
   * report 把一次用户操作上报服务端。
   * 只有带会话、且还在这条记录的过程上时才上报(换过随机条件就不是同一条过程了);
   * 失败不打断推演 —— 推演在本机是确定性的,上报是给服务端留记录,
   * 断网时把它当成一次失败的旁路而不是整场中断。
   */
  const report = useCallback(
    (eventType: string, payload: JsonObject, atTick: number) => {
      if (!sessionId || reportBlocked) return
      reportSeq.current += 1
      void api.sim
        .reportAction(sessionId, {
          seq: reportSeq.current,
          at_tick: atTick,
          event_type: eventType,
          payload,
        })
        .catch((reportError: unknown) => {
          console.error('[sim] 操作上报失败', { sessionId, eventType, error: reportError })
          toast.error('这一步没能记录到服务端,推演不受影响。')
        })
    },
    [reportBlocked, sessionId],
  )

  /** interact 注入一次用户操作:先进 Worker(它是权威),再上报。 */
  const interact = useCallback(
    (interaction: InteractionDescriptor, payload: JsonObject, target?: string) => {
      if (!client || !snapshot) return
      setPlaying(false)
      const body: JsonObject = target ? { ...payload, target } : payload
      sendCommand(client.inject(interaction.emits, body, target))
      report(interaction.emits, body, snapshot.tick)
    },
    [client, report, sendCommand, snapshot],
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

  const stepOnce = useCallback(() => {
    if (!client) return
    setPlaying(false)
    sendCommand(client.step())
  }, [client, sendCommand])

  /**
   * stepBack 回退一步:Worker 从初始状态重放到上一条事件,状态可复现,不是就地反算。
   * 带会话时回退即脱离服务端记录:那份记录是只追加的操作序列,表达不了「撤回一步」,
   * 之后继续上报会与已有条目撞号。故这里显式标记脱离,不再往记录里追加。
   */
  const stepBack = useCallback(() => {
    if (!client) return
    setPlaying(false)
    if (sessionId) setDiverged(true)
    sendCommand(client.back())
  }, [client, sendCommand, sessionId])

  /** restart 回到初始状态:同回退,带会话时一并脱离记录(不重置序号,记录不可改写)。 */
  const restart = useCallback(() => {
    if (!client) return
    setPlaying(false)
    if (sessionId) setDiverged(true)
    sendCommand(client.reset())
  }, [client, sendCommand, sessionId])

  // 运行环境装配失败时没有可看的画面,直接给出说明与唯一出口
  if (error && !snapshot) {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        title="这个场景没能跑起来"
        description={error}
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={exit}>
            返回仿真实验室
          </Button>
        }
      />
    )
  }

  if (!descriptor || !snapshot) {
    return (
      <AppStatusScreen
        icon={LoaderCircle}
        spinning
        title="正在装配仿真场景"
        fullScreen={false}
      />
    )
  }

  // 恢复现场时逐条重放已有操作,期间不接受新交互 —— 中途插入会打乱顺序
  if (restoring) {
    return (
      <AppStatusScreen
        icon={LoaderCircle}
        spinning
        title="正在恢复上次的推演"
        description={`按记录重放已经做过的 ${reportSeq.current} 步操作。`}
        fullScreen={false}
      />
    )
  }

  const frame = snapshot.view
  const narrativeDone = narrativeProgress(descriptor, snapshot)

  return (
    <>
      <WorkbenchShell
        topbar={
          <WorkbenchTopbar
            onExit={exit}
            exitLabel="退出推演"
            title={title}
            subtitle={version ? `${descriptor.meta.name} · ${version}` : descriptor.meta.name}
            progress={
              <ChainProgress
                onDark
                size="sm"
                label="教学步骤"
                total={descriptor.narrative.length}
                done={narrativeDone}
              />
            }
            cta={
              sessionId ? (
                <Button variant="seal" size="sm" leftIcon={Share2} onClick={() => setShareOpen(true)}>
                  分享这次推演
                </Button>
              ) : undefined
            }
          />
        }
        left={<TeachingFrameBrief frame={frame} />}
        leftLabel="场景说明"
        stage={
          <TeachingFrameStage
            frame={frame}
            selectedElementId={snapshot.state.selectedElementId}
            onSelectElement={selectElement}
          />
        }
        right={
          <div className="flex flex-col">
            <InteractionPanel
              interactions={descriptor.interactions}
              availability={snapshot.interactionAvailability}
              selectedElementId={snapshot.state.selectedElementId}
              onInteract={interact}
            />
            <CheckpointPanel descriptor={descriptor} snapshot={snapshot} />
            {frameHasAside(frame) ? (
              <TeachingFrameAside
                frame={frame}
                selectedElementId={snapshot.state.selectedElementId}
                onSelectElement={selectElement}
              />
            ) : null}
          </div>
        }
        rightLabel="操作与结论"
        footer={
          <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-2">
            <div className="flex min-w-0 flex-col">
              <span className="flex flex-wrap items-center gap-2 font-mono text-xs tabular-nums text-on-dark-sub">
                <span>推演时刻 {snapshot.tick}</span>
                <span>随机条件 {seed}</span>
                {sessionId ? (
                  reportBlocked ? (
                    <Badge tone="warning">已脱离记录</Badge>
                  ) : (
                    <Badge tone="jade">已记录 {reportSeq.current} 步</Badge>
                  )
                ) : (
                  <Badge tone="neutral">本机推演</Badge>
                )}
              </span>
              {error ? <span className="mt-0.5 text-xs text-on-dark-danger">{error}</span> : null}
              {sessionId && reportBlocked ? (
                <span className="mt-0.5 text-xs text-on-dark-sub">
                  回退、重播或换条件之后就不再往服务端记录里追加了 ——
                  那份记录是只往后写的操作序列,改不了已经写下的部分。分享出去的仍是记录里的过程。
                </span>
              ) : null}
            </div>
            <div className="flex shrink-0 flex-wrap items-center gap-2">
              <Button
                variant="on-dark"
                size="sm"
                leftIcon={Dices}
                disabled={Boolean(error)}
                onClick={() => setSeed((current) => current + 1)}
              >
                换一组条件
              </Button>
              <Button
                variant="on-dark"
                size="sm"
                leftIcon={RotateCcw}
                disabled={Boolean(error) || snapshot.events.length === 0}
                onClick={restart}
              >
                从头再来
              </Button>
              <Button
                variant="on-dark"
                size="sm"
                leftIcon={StepBack}
                disabled={Boolean(error) || snapshot.events.length === 0}
                onClick={stepBack}
              >
                上一步
              </Button>
              <Button
                variant="on-dark"
                size="sm"
                leftIcon={playing ? Pause : Play}
                disabled={Boolean(error)}
                onClick={() => setPlaying((current) => !current)}
              >
                {playing ? '暂停' : '自动推进'}
              </Button>
              <Button
                variant="on-dark"
                size="sm"
                leftIcon={StepForward}
                disabled={Boolean(error)}
                onClick={stepOnce}
              >
                下一步
              </Button>
            </div>
          </div>
        }
      />

      {shareOpen && sessionId ? (
        <ShareModal sessionId={sessionId} onClose={() => setShareOpen(false)} />
      ) : null}
    </>
  )
}

interface InteractionPanelProps {
  interactions: InteractionDescriptor[]
  availability: Record<string, boolean>
  selectedElementId?: string
  onInteract: (interaction: InteractionDescriptor, payload: JsonObject, target?: string) => void
}

/**
 * InteractionPanel 渲染场景声明的可用操作。
 * 能不能点由 Worker 算出的 interactionAvailability 决定 —— 场景自己知道当前状态允许什么,
 * 页面不复制一份判断逻辑。选元素类操作在舞台上直接点,不在这里重复出现按钮。
 */
function InteractionPanel({
  interactions,
  availability,
  selectedElementId,
  onInteract,
}: InteractionPanelProps) {
  const actionable = interactions.filter((item) => item.kind !== 'select-element')
  if (actionable.length === 0) return null

  return (
    <section className="flex flex-col gap-2 border-b border-dark-line p-4">
      <h2 className="text-sm font-medium text-on-dark">可用操作</h2>
      <ul className="flex flex-col gap-2">
        {actionable.map((interaction) => (
          <li key={interaction.id}>
            <InteractionItem
              interaction={interaction}
              enabled={availability[interaction.id] !== false}
              selectedElementId={selectedElementId}
              onInteract={onInteract}
            />
          </li>
        ))}
      </ul>
    </section>
  )
}

interface InteractionItemProps {
  interaction: InteractionDescriptor
  enabled: boolean
  selectedElementId?: string
  onInteract: (interaction: InteractionDescriptor, payload: JsonObject, target?: string) => void
}

/**
 * InteractionItem 渲染一个操作及其参数。
 * 参数按场景声明的字段类型渲染显式控件,默认值取声明里的 default ——
 * 这些字段是场景作者定义的教学变量,不是自由输入。
 */
function InteractionItem({
  interaction,
  enabled,
  selectedElementId,
  onInteract,
}: InteractionItemProps) {
  const fields = interaction.params ?? []
  const [values, setValues] = useState<JsonObject>(() =>
    Object.fromEntries(fields.map((field) => [field.name, field.default])),
  )

  const needsElement = interaction.target === 'element'
  const blocked = !enabled || (needsElement && !selectedElementId)

  return (
    <div className="flex flex-col gap-2 rounded-md border border-dark-line bg-dark-surface p-2">
      <div className="flex items-start justify-between gap-2">
        <span className="min-w-0 text-sm text-on-dark">{interaction.label}</span>
        {interaction.labelTag && interaction.labelTag !== 'normal' ? (
          <Badge tone={interaction.labelTag === 'attack' ? 'cinnabar' : 'warning'}>
            {TAG_LABELS[interaction.labelTag]}
          </Badge>
        ) : null}
      </div>
      {interaction.description ? (
        <p className="text-xs text-on-dark-sub">{interaction.description}</p>
      ) : null}

      {fields.map((field) => (
        <label key={field.name} className="flex flex-col gap-1">
          <span className="text-xs text-on-dark-sub">{field.label}</span>
          {field.type === 'boolean' ? (
            <input
              type="checkbox"
              checked={values[field.name] === true}
              onChange={(event) =>
                setValues((current) => ({ ...current, [field.name]: event.target.checked }))
              }
              className="size-4 accent-jade-500"
            />
          ) : field.type === 'select' ? (
            <select
              value={String(values[field.name] ?? '')}
              onChange={(event) =>
                setValues((current) => ({
                  ...current,
                  [field.name]: optionValue(field.options, event.target.value),
                }))
              }
              className="rounded-md border border-dark-line bg-dark-elevated px-2 py-1 text-sm text-on-dark focus:border-accent focus:outline-none"
            >
              {(field.options ?? []).map((option) => (
                <option key={String(option.value)} value={String(option.value)}>
                  {option.label}
                </option>
              ))}
            </select>
          ) : (
            <input
              type={field.type === 'string' ? 'text' : 'number'}
              value={String(values[field.name] ?? '')}
              min={field.min}
              max={field.max}
              step={field.step}
              onChange={(event) =>
                setValues((current) => ({
                  ...current,
                  [field.name]:
                    field.type === 'string' ? event.target.value : Number(event.target.value),
                }))
              }
              className="rounded-md border border-dark-line bg-dark-elevated px-2 py-1 font-mono text-sm text-on-dark focus:border-accent focus:outline-none"
            />
          )}
        </label>
      ))}

      <div>
        <Button
          variant="seal"
          size="sm"
          disabled={blocked}
          onClick={() => onInteract(interaction, values, needsElement ? selectedElementId : undefined)}
        >
          执行
        </Button>
      </div>
      {needsElement && !selectedElementId ? (
        <p className="text-xs text-on-dark-faint">先在舞台上点一个元素,再执行这个操作。</p>
      ) : null}
    </div>
  )
}

interface CheckpointPanelProps {
  descriptor: SimPackageDescriptor
  snapshot: RuntimeSnapshot
}

/**
 * CheckpointPanel 展示场景自带的教学检查点结论。
 * 这些是场景作者写在包里的判定(达成/未达成 + 解释),不是平台判分 ——
 * 实验里的判分走检查点判分接口,两者不混。
 */
function CheckpointPanel({ descriptor, snapshot }: CheckpointPanelProps) {
  if (descriptor.checkpoints.length === 0) return null

  return (
    <section className="flex flex-col gap-2 border-b border-dark-line p-4">
      <h2 className="text-sm font-medium text-on-dark">观察结论</h2>
      <ul className="flex flex-col gap-2">
        {descriptor.checkpoints.map((checkpoint) => {
          const result = snapshot.checkpointResults[checkpoint.id]
          return (
            <li
              key={checkpoint.id}
              className="flex flex-col gap-1 rounded-md border border-dark-line bg-dark-surface p-2"
            >
              <div className="flex items-start justify-between gap-2">
                <span className="min-w-0 text-sm text-on-dark">{checkpoint.label}</span>
                <Badge tone={result?.achieved ? 'success' : 'neutral'}>
                  {result?.achieved ? '已观察到' : '尚未出现'}
                </Badge>
              </div>
              {result?.explanation ? (
                <p className="text-xs text-on-dark-sub">{result.explanation}</p>
              ) : null}
            </li>
          )
        })}
      </ul>
    </section>
  )
}

interface ShareModalProps {
  sessionId: string
  onClose: () => void
}

/**
 * ShareModal 为这次推演生成公开分享码。
 * 分享出去的是「场景 + 种子 + 操作序列」,任何人打开都能复现同一过程;
 * 它不携带账号与学校信息,也不是取运行包的凭据(公开回放只放平台内置场景)。
 */
function ShareModal({ sessionId, onClose }: ShareModalProps) {
  const [expireAt, setExpireAt] = useState('')
  const [code, setCode] = useState<string>()
  const [formError, setFormError] = useState<string>()
  const [working, setWorking] = useState(false)

  const submit = useCallback(async () => {
    setFormError(undefined)
    setWorking(true)
    try {
      const result = await api.sim.shareSession(sessionId, {
        expire_at: expireAt ? new Date(expireAt).toISOString() : undefined,
      })
      setCode(result.code)
      toast.success('分享码已生成')
    } catch (error) {
      setFormError(userFacingErrorMessage(error, '分享码没有生成成功,请稍后重试。'))
    } finally {
      setWorking(false)
    }
  }, [expireAt, sessionId])

  const link = code ? `${window.location.origin}/sim/shared/${code}` : ''

  return (
    <Modal open onOpenChange={(open) => !open && onClose()}>
      <ModalContent size="lg">
        <ModalHeader>
          <ModalTitle>分享这次推演</ModalTitle>
          <ModalDescription>
            分享的是这次推演的过程本身:场景、随机条件与你的操作序列。任何人打开都能看到同一过程,
            链接里不含你的账号与学校信息。
          </ModalDescription>
        </ModalHeader>
        <ModalBody className="flex flex-col gap-4">
          {code ? (
            <div className="flex flex-col gap-2 rounded-md border border-line bg-surface-sunken p-3">
              <span className="text-sm text-ink-sub">公开回放链接</span>
              <span className="break-all font-mono text-sm text-ink">{link}</span>
              <Button
                variant="outline"
                size="sm"
                leftIcon={Link2}
                onClick={() => {
                  void navigator.clipboard
                    .writeText(link)
                    .then(() => toast.success('链接已复制'))
                    .catch(() => toast.error('复制没有成功,请手动选中链接。'))
                }}
              >
                复制链接
              </Button>
            </div>
          ) : (
            <label className="flex flex-col gap-1">
              <span className="text-sm text-ink">有效期至</span>
              <input
                type="datetime-local"
                value={expireAt}
                onChange={(event) => setExpireAt(event.target.value)}
                className="rounded-md border border-line-strong bg-surface px-3 py-2 text-base text-ink focus:border-primary focus:outline-none"
              />
              <span className="text-xs text-ink-sub">留空表示按平台默认有效期。</span>
            </label>
          )}

          {formError ? <p className="text-sm text-danger">{formError}</p> : null}
        </ModalBody>
        <ModalFooter>
          <Button variant="outline" onClick={onClose}>
            {code ? '关闭' : '取消'}
          </Button>
          {code ? null : (
            <Button variant="seal" leftIcon={Share2} loading={working} onClick={() => void submit()}>
              生成分享码
            </Button>
          )}
        </ModalFooter>
      </ModalContent>
    </Modal>
  )
}

/**
 * narrativeProgress 把当前叙事步骤换成链式进度的完成数:讲到第几步,前面几步就都讲过了。
 * 叙事步骤由状态断言命中,尚未命中任何一步时进度为 0。
 */
function narrativeProgress(descriptor: SimPackageDescriptor, snapshot: RuntimeSnapshot): number {
  const currentId = snapshot.currentStep?.id
  return descriptor.narrative.findIndex((step) => step.id === currentId) + 1
}

/** simStepIntervalMs 自动推进的步长:与公开回放同频,保证同一场景两处看起来节奏一致。 */
function simStepIntervalMs(): number {
  return 1200
}

/** optionValue 从声明的选项里取回原始值:select 的 DOM 值是字符串,不能直接当业务值用。 */
function optionValue(
  options: Array<{ label: string; value: JsonValue }> | undefined,
  raw: string,
): JsonValue {
  const matched = (options ?? []).find((option) => String(option.value) === raw)
  return matched ? matched.value : raw
}
