// replayMoves 把服务端记录的动作序列摊平成逐步可控的 Worker 命令。
//
// 为什么必须摊平:每条动作发生在特定推演时刻,而 Worker 只按命令推进时刻
// (sim.worker.ts 的 applyEvent 仅在 tick 事件后加一),所以注入之前必须先把时刻走到位 ——
// 顺序错了复现出的就不是原过程。摊平后一次命令对应 Worker 内一个事件,
// 进度、单步与后退共用同一把尺子。
//
// 公开回放与带会话的仿真工作台都要做这件事(一个是复现别人的记录,一个是恢复自己的现场),
// 故收敛到这一处:同一件事一个实现。

import type { JsonObject } from '@chaimir/sim-sdk'
import type { SimWorkerClient } from '@chaimir/sim-sdk'
import type { SimActionLog } from '@chaimir/api-client'

/** ReplayCommand 是一条待复现的记录动作:Worker 事件入参 + 它发生时的推演时刻。 */
interface ReplayCommand {
  eventType: string
  atTick: number
  payload: JsonObject
  target?: string
}

/** ReplayMove 是复现记录时的一次 Worker 命令:推进一个推演时刻,或注入一条记录动作。 */
type ReplayMove = { kind: 'tick' } | { kind: 'action'; command: ReplayCommand }

/**
 * replayCommands 在 DTO 边界把后端动作日志收敛成 Worker 事件入参。
 * payload 是仿真包写下的 JSON 对象;元素目标由 Worker 在记录时写进 payload.target
 * (见 sim-sdk 的 sim.worker.ts userEventInput),复现时按同一约定取回顶层 target。
 * 后端已保证动作按 seq 连续、at_tick 单调不减,这里不排序也不补洞。
 */
function replayCommands(actions: SimActionLog[]): ReplayCommand[] {
  return actions.map((action) => {
    const payload = action.payload as JsonObject
    return {
      eventType: action.event_type,
      atTick: action.at_tick,
      payload,
      target: payload.target as string | undefined,
    }
  })
}

/**
 * replayMoves 把记录动作摊平成命令序列。
 */
export function replayMoves(actions: SimActionLog[]): ReplayMove[] {
  const moves: ReplayMove[] = []
  let tick = 0
  for (const command of replayCommands(actions)) {
    while (tick < command.atTick) {
      moves.push({ kind: 'tick' })
      tick += 1
    }
    moves.push({ kind: 'action', command })
  }
  return moves
}

/**
 * moveCommand 把一条复现命令交给 Worker:推进时刻用 step,复现动作用 inject。
 * 元素目标按 Worker 的记录约定从 payload.target 取回并同时作为顶层 target 传入,
 * 与当初记录这条动作时的入参完全一致(见 sim.worker.ts userEventInput)。
 */
export function moveCommand(client: SimWorkerClient, move: ReplayMove): Promise<void> {
  if (move.kind === 'tick') {
    return client.step()
  }
  return client.inject(move.command.eventType, move.command.payload, move.command.target)
}
