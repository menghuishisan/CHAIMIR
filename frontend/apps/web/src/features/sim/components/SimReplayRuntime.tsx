// 内置包的公开分享和租户内归档共用只读回放；只执行记录中的事件。
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { LoaderCircle, TriangleAlert } from 'lucide-react'
import { Badge, Button, useReducedMotion } from '@chaimir/ui'
import { SimWorkerClient } from '@chaimir/sim-sdk'
import type { RuntimeSnapshot, SimInitParams, SimPackageDescriptor } from '@chaimir/sim-sdk'
import type { SimReplay } from '@chaimir/api-client'
import { appConfig } from '../../../app/config'
import { AppStatusScreen } from '../../../components/AppStatusScreen'
import { errorDiagnostics } from '../../../utils/userFacingError'
import { moveCommand, replayMoves } from '../replayMoves'
import { useSimPlayback } from '../playback'
import { SimPlaybackControls } from './SimPlaybackControls'
import { SimRuntimeWorkbench } from './SimRuntimeWorkbench'

interface SimReplayRuntimeProps {
  replay: SimReplay
  title: string
  onExit: () => void
  action?: ReactNode
}

/** SimReplayRuntime 按记录播放内置包，公开与校内回放共用同一组控制。 */
export function SimReplayRuntime({ replay, title, onExit, action }: SimReplayRuntimeProps) {
  const reducedMotion = useReducedMotion()
  const moves = useMemo(() => replayMoves(replay.actions), [replay.actions])
  const [client, setClient] = useState<SimWorkerClient>()
  const [descriptor, setDescriptor] = useState<SimPackageDescriptor>()
  const [snapshot, setSnapshot] = useState<RuntimeSnapshot>()
  const [error, setError] = useState<string>()
  const pending = useRef(false)
  const sendCommand = useCallback((command: () => Promise<void>) => {
    if (pending.current) return
    pending.current = true
    void command().catch((cause: unknown) => {
      console.error('仿真回放失败', { operation: 'sim.replay.command', error: errorDiagnostics(cause) })
      setError('推演没能继续，请重新打开后重试。')
    }).finally(() => { pending.current = false })
  }, [])

  useEffect(() => {
    let active = true
    const fail = (cause: unknown) => {
      if (!active) return
      console.error('仿真回放运行失败', { operation: 'sim.replay.runtime', error: errorDiagnostics(cause) })
      setError('推演没能继续，请重新打开后重试。')
    }
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
      onSnapshot: (next) => {
        if (active) setSnapshot(next)
      },
      onError: fail,
    })
    void runtime.init().then(() => {
      if (active) setClient(runtime)
    }).catch(fail)
    return () => {
      active = false
      runtime.destroy()
    }
  }, [replay])

  // 事件日志是唯一游标，后退或从头回放不会产生第二份进度状态。
  const cursor = snapshot?.events.length ?? 0
  const finished = cursor >= moves.length
  const advance = useCallback(() => {
    if (client && !finished) sendCommand(() => moveCommand(client, moves[cursor]))
  }, [client, cursor, finished, moves, sendCommand])
  const playback = useSimPlayback({
    advance,
    canAdvance: Boolean(client) && !error && !finished,
    cue: snapshot,
    autoPlay: !reducedMotion,
  })
  const step = () => {
    playback.stop()
    advance()
  }
  const back = () => {
    playback.stop()
    if (client) sendCommand(() => client.back())
  }
  const restart = () => {
    playback.stop()
    if (client) sendCommand(() => client.reset())
  }

  if (error && !snapshot) {
    return (
      <AppStatusScreen
        icon={TriangleAlert}
        tone="danger"
        title="这段推演没能打开"
        description={error}
        fullScreen={false}
        actions={<Button variant="on-dark" onClick={onExit}>退出回放</Button>}
      />
    )
  }
  if (!descriptor || !snapshot) {
    return <AppStatusScreen icon={LoaderCircle} spinning title="正在准备回放" fullScreen={false} />
  }
  return (
    <SimRuntimeWorkbench
      title={title}
      version={replay.version}
      descriptor={descriptor}
      snapshot={snapshot}
      onExit={onExit}
      titleExtra={<Badge onDark tone="neutral">只读回放</Badge>}
      action={action}
      footer={
        <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 px-4 py-2">
          <div className="flex min-w-0 flex-col">
            <span className="font-mono text-xs tabular-nums text-on-dark-sub">
              推演时刻 {snapshot.tick} · 已复现操作 {snapshot.events.filter((event) => event.source === 'user').length}/{replay.actions.length}
            </span>
            {error ? <span className="mt-0.5 text-xs text-on-dark-danger">{error}</span> : null}
          </div>
          <SimPlaybackControls
            playing={playback.playing}
            onToggle={playback.toggle}
            onStep={step}
            onStepBack={back}
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
