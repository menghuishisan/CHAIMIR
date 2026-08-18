// 隔离执行流:本校自建场景在服务端容器里运行,本文件把 WebSocket 帧翻译成与本机推演同形的运行时快照。
//
// 为什么要翻译:扩展场景的 render 也是外部代码,浏览器不执行它,故容器算完整教学帧后回传
// (见 docs/04-仿真可视化引擎/02-架构设计.md §8)。帧的字段名是服务端下划线口径,
// 而工作台的面板与本机推演共用 —— 在这一处收敛成同一个形状,页面就只有一条渲染路径,
// 不为「跑在哪」写第二套舞台、时间线与检查器。
//
// 首帧(type=ready)带包自描述信息:操作清单、教学步骤、检查点标题与代码追踪都在包里声明,
// 浏览器不执行包代码就拿不到,只能由容器给出;它已在后端按包的交互白名单核对过。

import { useCallback, useMemo, useState } from 'react'
import type {
  CheckpointResult,
  JsonObject,
  NarrativeStepDescriptor,
  RuntimeSnapshot,
  SimPackageDescriptor,
  SimState,
  TeachingFrame,
} from '@chaimir/sim-sdk'
import {
  SIM_STREAM_COMMAND,
  SIM_STREAM_FRAME,
  type SimStreamCommandMessage,
  type SimStreamFrame,
  type SimTeachingSnapshot,
} from '@chaimir/api-client'
import { api } from '../../app/api'
import { useTicketedWebSocket, type SocketStatus } from '../../hooks/useTicketedWebSocket'

/**
 * IsolatedSnapshot 是隔离执行的一帧,与本机推演的运行时快照同形。
 * 少 events:过程由服务端持有(容器每次从初始状态重放),页面不需要也不应该再存一份。
 */
export type IsolatedSnapshot = Omit<RuntimeSnapshot, 'events'>

export interface IsolatedStreamState {
  status: SocketStatus
  /** 用户向失败文案;技术细节进控制台 */
  error?: string
  descriptor?: SimPackageDescriptor
  snapshot?: IsolatedSnapshot
  /** 当前过程已执行的事件数:过程由服务端持有,前端自己数不准(刷新重连后服务端会重放回来) */
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
  descriptor?: SimPackageDescriptor
  snapshot: SimTeachingSnapshot
  event_count?: number
}

/**
 * useIsolatedSimStream 建立隔离执行连接并维护最近一帧。
 * 建连走全站唯一的换票实现(useTicketedWebSocket),不在这里另写一套 WebSocket 生命周期。
 */
export function useIsolatedSimStream(sessionId: string): IsolatedStreamState {
  const [descriptor, setDescriptor] = useState<SimPackageDescriptor>()
  const [snapshot, setSnapshot] = useState<IsolatedSnapshot>()
  const [eventCount, setEventCount] = useState(0)
  const [frameError, setFrameError] = useState<string>()

  const url = useMemo(() => api.sim.getStreamWsUrl(sessionId), [sessionId])

  const handleMessage = useCallback((raw: string) => {
    const message = parseStreamMessage(raw)
    if (!message) {
      setFrameError('这个场景的运行画面没能解析,请稍后重试。')
      return
    }
    if (message.descriptor) {
      setDescriptor(message.descriptor)
    }
    setEventCount(message.event_count ?? 0)
    setSnapshot(toIsolatedSnapshot(message.snapshot))
  }, [])

  const socket = useTicketedWebSocket({ url, onMessage: handleMessage })

  /** send 把一条受控命令按后端契约发出;命令类型只取 SIM_STREAM_COMMAND 登记的四种。 */
  const send = useCallback(
    (message: SimStreamCommandMessage) => socket.send(JSON.stringify(message)),
    [socket],
  )

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
    error: socket.error ?? frameError,
    descriptor,
    snapshot,
    eventCount,
    step,
    inject,
    back,
    restart,
    reconnect: socket.reconnect,
  }
}

/**
 * parseStreamMessage 解析一帧文本;结构不符即返回 undefined 交由调用方给用户向提示。
 */
function parseStreamMessage(raw: string): StreamMessage | undefined {
  try {
    const parsed = JSON.parse(raw) as Partial<StreamMessage>
    if (!parsed || typeof parsed !== 'object' || !parsed.snapshot) {
      return undefined
    }
    return {
      type: parsed.type === SIM_STREAM_FRAME.READY ? SIM_STREAM_FRAME.READY : SIM_STREAM_FRAME.SNAPSHOT,
      descriptor: parsed.descriptor,
      snapshot: parsed.snapshot,
      event_count: typeof parsed.event_count === 'number' ? parsed.event_count : 0,
    }
  } catch (error) {
    console.error('[sim] 隔离执行帧解析失败', { kind: error instanceof Error ? error.name : typeof error })
    return undefined
  }
}

/**
 * toIsolatedSnapshot 把服务端下划线字段转成运行时快照形状。
 *
 * 这里的类型断言不是"盲信 JSON":帧在转发前已由后端按 TeachingFrame 协议逐项校验
 * (封闭模式数量与取值、layout 引用完整性、规模上限,见 backend validation_frame.go),
 * 前端再重复一遍同样的规则只会得到两份必然漂移的实现。
 */
function toIsolatedSnapshot(raw: SimTeachingSnapshot): IsolatedSnapshot {
  return {
    tick: raw.tick,
    state: raw.state as unknown as SimState,
    view: raw.view as unknown as TeachingFrame,
    currentStep: raw.current_step as unknown as NarrativeStepDescriptor | undefined,
    interactionAvailability: raw.interaction_availability ?? {},
    checkpointResults: (raw.checkpoint_results ?? {}) as unknown as Record<string, CheckpointResult>,
  }
}
