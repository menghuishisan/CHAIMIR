// 将受控隔离流转换为共享工作台快照；连接开放不代表运行时首帧已就绪。

import { useCallback, useRef, useState } from 'react'
import type {
  JsonObject,
  RuntimeFrame,
  SimPackageDescriptor,
} from '@chaimir/sim-sdk'
import { isRuntimeFrame, isSimPackageDescriptor } from '@chaimir/sim-sdk'
import {
  SIM_STREAM_COMMAND,
  SIM_STREAM_FRAME,
  type SimStreamCommandMessage,
  type SimStreamFrame,
} from '@chaimir/api-client'
import { api } from '../../app/api'
import { useTicketedWebSocket, type SocketStatus } from '../../hooks/useTicketedWebSocket'

export interface IsolatedStreamState {
  status: SocketStatus
  readOnly: boolean
  /** 用户向失败文案;技术细节进控制台 */
  error?: string
  descriptor?: SimPackageDescriptor
  snapshot?: RuntimeFrame
  /** 含重连恢复的服务端事件数。 */
  eventCount: number
  /** 推进一个推演时刻 */
  step: () => void
  /** 注入一次场景声明的操作 */
  inject: (eventType: string, payload: JsonObject, target?: string) => void
  /** 回退一步(服务端按确定性重算到上一时刻) */
  back: () => void
  /** 回到初始状态 */
  restart: () => void
  reconnect: () => void
}

/** StreamMessage 是服务端推来的一帧,字段名与后端 BackendStreamMessage 一致。 */
interface StreamMessage {
  type: SimStreamFrame
  read_only: boolean
  descriptor?: SimPackageDescriptor
  snapshot: RuntimeFrame
  event_count: number
}

/** 复用统一换票连接，只在首帧就绪且上条命令已确认后开放发送。 */
export function useIsolatedSimStream(sessionId: string): IsolatedStreamState {
  const [frame, setFrame] = useState<StreamMessage>()
  const [frameError, setFrameError] = useState<string>()
  const commandState = useRef<'waiting' | 'ready' | 'pending'>('waiting')
  const url = api.sim.getStreamWsUrl(sessionId)

  const handleMessage = useCallback((raw: string) => {
    const message = parseStreamMessage(raw)
    if (!message || (commandState.current === 'waiting' && message.type !== SIM_STREAM_FRAME.READY)) {
      commandState.current = 'waiting'
      setFrameError('这个场景的运行画面没能解析，请重新连接。')
      return
    }
    commandState.current = 'ready'
    setFrameError(undefined)
    setFrame((previous) => ({ ...message, descriptor: message.descriptor ?? previous?.descriptor }))
  }, [])
  const reset = useCallback(() => {
    commandState.current = 'waiting'
    setFrame(undefined)
    setFrameError(undefined)
  }, [])
  const socket = useTicketedWebSocket({ url, onMessage: handleMessage, onOpen: reset })
  const { send: sendText, reconnect: reconnectSocket } = socket
  const send = useCallback((message: SimStreamCommandMessage) => {
    if (commandState.current !== 'ready') return
    if (sendText(JSON.stringify(message))) commandState.current = 'pending'
  }, [sendText])
  const reconnect = useCallback(() => {
    reset()
    reconnectSocket()
  }, [reset, reconnectSocket])

  const step = useCallback(() => send({ type: SIM_STREAM_COMMAND.STEP }), [send])
  const back = useCallback(() => send({ type: SIM_STREAM_COMMAND.BACK }), [send])
  const restart = useCallback(() => send({ type: SIM_STREAM_COMMAND.RESTART }), [send])
  const inject = useCallback(
    (eventType: string, payload: JsonObject, target?: string) => {
      const body: JsonObject = target ? { ...payload, target } : payload
      send({ type: SIM_STREAM_COMMAND.EVENT, event_type: eventType, payload: body })
    },
    [send],
  )

  return {
    status: socket.status,
    readOnly: frame?.read_only ?? true,
    error: socket.error ?? frameError ?? (socket.status === 'closed' ? '连接已断开，请重新连接。' : undefined),
    descriptor: frame?.descriptor,
    snapshot: frame?.snapshot,
    eventCount: frame?.event_count ?? 0,
    step,
    inject,
    back,
    restart,
    reconnect,
  }
}

function parseStreamMessage(raw: string): StreamMessage | undefined {
  try {
    const parsed = JSON.parse(raw) as Partial<StreamMessage>
    if (!parsed || typeof parsed !== 'object' || !parsed.snapshot || typeof parsed.read_only !== 'boolean'
      || (parsed.type !== SIM_STREAM_FRAME.READY && parsed.type !== SIM_STREAM_FRAME.SNAPSHOT)
      || (parsed.type === SIM_STREAM_FRAME.READY && !isSimPackageDescriptor(parsed.descriptor))
      || !Number.isSafeInteger(parsed.event_count) || (parsed.event_count ?? -1) < 0) {
      return undefined
    }
    const wire = parsed.snapshot as unknown as Record<string, unknown>
    const snapshot = {
      tick: wire.tick, state: wire.state, view: wire.view,
      currentStep: wire.current_step,
      interactionAvailability: wire.interaction_availability,
      checkpointResults: wire.checkpoint_results,
    }
    if (!isRuntimeFrame(snapshot)) return undefined
    return {
      type: parsed.type,
      read_only: parsed.read_only === true,
      descriptor: parsed.type === SIM_STREAM_FRAME.READY ? parsed.descriptor : undefined,
      snapshot,
      event_count: parsed.event_count as number,
    }
  } catch (error) {
    console.error('[sim] 隔离执行帧解析失败', { kind: error instanceof Error ? error.name : typeof error })
    return undefined
  }
}
