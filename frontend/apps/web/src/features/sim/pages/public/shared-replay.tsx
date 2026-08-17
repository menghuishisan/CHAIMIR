// shared-replay 是公开仿真回放页(免认证深链 /sim/shared/:shareCode,全屏沉浸态)。
// 页面做三件事:取回分享的推演记录、在隔离 Worker 里按记录逐条复现、把教学帧铺进工作台三区。
//
// 匿名边界:全平台唯一无会话业务接口是 GET sim/shared/:code,本页只调它 ——
//   不上报动作、不换取运行包凭据、不建立实时连接。
// 运行边界:公开链接只回放平台内置场景。扩展场景的运行包属所属学校的教学资产,
//   分享码不是跨校取包凭据,因此非内置场景在此说明不支持,不做降级渲染。
// 数据边界:只呈现推演过程本身,不显示记录时间、所属学校与会话归属。

import { useCallback, useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router'
import { Info, LoaderCircle, Lock, TriangleAlert } from 'lucide-react'
import {
  Button,
  ChainProgress,
  TeachingFrameStage,
  TeachingFrameStream,
  WorkbenchShell,
  WorkbenchTopbar,
  frameStreamEntries,
  frameHasStream,
  useReducedMotion,
} from '@chaimir/ui'
import { SimWorkerClient, isBuiltinSimulationCode } from '@chaimir/sim-sdk'
import type { RuntimeSnapshot, SimInitParams, SimPackageDescriptor } from '@chaimir/sim-sdk'
import type { SimReplay } from '@chaimir/api-client'
import { api } from '../../../../app/api'
import { appConfig } from '../../../../app/config'
import { AppStatusScreen } from '../../../../components/AppStatusScreen'
import { useAsyncResource } from '../../../../hooks/useAsyncResource'
import { useImmersive } from '../../../../layouts/immersive/context'
import { moveCommand, replayMoves } from '../../replayMoves'
import { narrativeProgress, useSimPlayback } from '../../playback'
import { SimPlaybackControls } from '../../SimPlaybackControls'
import { SimWorkbenchAside } from '../../SimWorkbenchAside'

/** injectedActionCount 数出快照里已复现的记录动作数(记录动作在 Worker 内是 user 事件)。 */
function injectedActionCount(snapshot: RuntimeSnapshot): number {
  return snapshot.events.reduce((total, event) => total + (event.source === 'user' ? 1 : 0), 0)
}

/**
 * PublicReplayPage 取回分享记录并判定可回放边界,自身不接触仿真运行时。
 */
export default function PublicReplayPage() {
  const { shareCode = '' } = useParams<{ shareCode: string }>()
  const { exit } = useImmersive()
  const replay = useAsyncResource(
    () => api.sim.getSharedReplay(shareCode),
    [shareCode],
    (value) => value.actions.length === 0,
  )

  if (replay.status === 'loading') {
    return <AppStatusScreen icon={LoaderCircle} spinning title="正在打开分享的推演" fullScreen={false} />
  }

  if (replay.status === 'error' || !replay.data) {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        tone="danger"
        title="暂时打不开这段推演"
        description={replay.error?.message}
        traceId={replay.error?.traceId}
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={replay.reload}>
            重新加载
          </Button>
        }
      />
    )
  }

  if (replay.status === 'empty') {
    return (
      <AppStatusScreen
        icon={Info}
        title="这段分享里没有可回放的操作"
        description="分享者在这个场景里没有留下操作记录。"
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={exit}>
            返回登录
          </Button>
        }
      />
    )
  }

  // 运行边界:公开链接只回放平台内置场景。扩展场景的运行包要凭校内会话取回,
  // 分享码不承担这个授权,故在此说明清楚,不引导访客去换取运行授权。
  if (!isBuiltinSimulationCode(replay.data.package_code)) {
    return (
      <AppStatusScreen
        icon={Lock}
        title="这段推演不支持公开回放"
        description="分享的是本校自建的仿真场景,需要登录校内账号后在仿真工作台中查看。"
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={exit}>
            返回登录
          </Button>
        }
      />
    )
  }

  return <ReplayRuntime replay={replay.data} />
}

interface ReplayRuntimeProps {
  replay: SimReplay
}

/**
 * ReplayRuntime 持有 Worker 生命周期与复现进度,并把每个快照铺进工作台三栏。
 * 进度不另存一份:Worker 内的事件日志才是权威,复现到第几步就是 snapshot.events.length ——
 * 页面另记一个游标只会与后退、重播产生漂移。
 */
function ReplayRuntime({ replay }: ReplayRuntimeProps) {
  const { title, exit } = useImmersive()
  const reducedMotion = useReducedMotion()
  const moves = useMemo(() => replayMoves(replay.actions), [replay.actions])
  const [client, setClient] = useState<SimWorkerClient>()
  const [descriptor, setDescriptor] = useState<SimPackageDescriptor>()
  const [snapshot, setSnapshot] = useState<RuntimeSnapshot>()
  const [error, setError] = useState<string>()

  /**
   * sendCommand 发出一条 Worker 命令并把失败原因记为页面错误态。
   * 客户端在拒绝之前已把技术细节写进控制台、只留用户向文案(SimWorkerClient),
   * 故这里直接采用它的 message;onError 与拒绝是同一失败的两个通知口,写入同一处状态。
   */
  const sendCommand = useCallback((command: Promise<void>) => {
    void command.catch((commandError: Error) => setError(commandError.message))
  }, [])

  // Worker 与这段记录同生共死。这里不做「只装配一次」的守卫:清理已终止上一个 Worker,
  // 严格模式的二次挂载必须重新装配,跳过就只剩一个已终止的 Worker。
  useEffect(() => {
    let active = true
    const runtime = new SimWorkerClient({
      builtinCode: replay.package_code,
      initParams: replay.init_params as SimInitParams,
      seed: replay.seed,
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
    sendCommand(runtime.init())
    return () => {
      active = false
      runtime.destroy()
    }
  }, [replay, sendCommand])

  const cursor = snapshot ? snapshot.events.length : 0
  const finished = cursor >= moves.length

  /** advance 复现下一条记录命令(推进时刻或注入动作)。 */
  const advance = useCallback(() => {
    if (!client || finished) return
    sendCommand(moveCommand(client, moves[cursor]))
  }, [client, cursor, finished, moves, sendCommand])

  // 减弱动效偏好下不自动播放(规范 §7.3:冻结并保留当前态文字),单步复现仍然可用
  const playback = useSimPlayback({
    advance,
    canAdvance: Boolean(client) && !error && !finished,
    cue: snapshot,
    autoPlay: !reducedMotion,
  })

  /** 单步即接管:手动走一步就停下自动复现,让访客按自己的节奏看。 */
  const stepOnce = useCallback(() => {
    playback.stop()
    advance()
  }, [advance, playback])

  /** 回退一步:Worker 从初始状态重放到上一条事件,状态可复现,不是就地反算。 */
  const stepBack = useCallback(() => {
    if (!client) return
    playback.stop()
    sendCommand(client.back())
  }, [client, playback, sendCommand])

  /** 从头再看:回到初始状态,进度与推演时刻一并归零。 */
  const restart = useCallback(() => {
    if (!client) return
    playback.stop()
    sendCommand(client.reset())
  }, [client, playback, sendCommand])

  // 运行环境装配失败时没有可看的画面,直接给出说明与唯一出口
  if (error && !snapshot) {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        tone="danger"
        title="这段推演没能跑起来"
        description={error}
        fullScreen={false}
        actions={
          <Button variant="on-dark" onClick={exit}>
            返回登录
          </Button>
        }
      />
    )
  }

  if (!descriptor || !snapshot) {
    return <AppStatusScreen icon={LoaderCircle} spinning title="正在准备回放" fullScreen={false} />
  }

  const frame = snapshot.view
  // 事件流:只有会产生消息/调用的场景才挂这一栏;条数供壳在收起的把手上给未读计数
  const hasStream = frameHasStream(frame)
  const streamCount = frameStreamEntries(frame).length

  return (
    <WorkbenchShell
      workbench="sim"
      topbar={
        <WorkbenchTopbar
          onExit={exit}
          exitLabel="退出回放"
          title={title}
          subtitle={descriptor.meta.name}
          progress={
            <ChainProgress
              onDark
              size="sm"
              label="教学步骤"
              total={descriptor.narrative.length}
              done={narrativeProgress(descriptor, snapshot)}
            />
          }
        />
      }
      // 左辅助区与登录后同构:阶段说明 / 观察结论 / 执行追踪 / 证据,只是没有操作区 ——
      // 公开回放不接受访客操作,访客看到的教学内容与登录后一致
      left={
        <SimWorkbenchAside
          frame={frame}
          checkpoints={descriptor.checkpoints}
          checkpointResults={snapshot.checkpointResults}
          codeTrace={descriptor.codeTrace}
          trace={snapshot.state._trace}
          selectedElementId={snapshot.state.selectedElementId}
        />
      }
      leftLabel="说明"
      // 只读舞台:公开回放不接受访客操作,故不传 onSelectElement;
      // 高亮跟随记录本身 —— 选中态由仿真包写在状态里,是原操作者当时的选择
      stage={<TeachingFrameStage frame={frame} selectedElementId={snapshot.state.selectedElementId} />}
      right={
        hasStream ? (
          <TeachingFrameStream frame={frame} selectedElementId={snapshot.state.selectedElementId} />
        ) : undefined
      }
      rightLabel="消息流"
      rightCount={streamCount}
      footer={
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-2">
          <div className="flex min-w-0 flex-col">
            <span className="font-mono text-xs tabular-nums text-on-dark-sub">
              推演时刻 {snapshot.tick} · 已复现操作 {injectedActionCount(snapshot)}/{replay.actions.length}
            </span>
            {/* 运行中出错:复现已停在当前一步,原因用后端/运行时给出的用户向文案,技术细节只进控制台 */}
            {error ? <span className="mt-0.5 text-xs text-on-dark-danger">{error}</span> : null}
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
            atEnd={finished}
            atStart={cursor === 0}
            playLabel="播放"
          />
        </div>
      }
    />
  )
}
