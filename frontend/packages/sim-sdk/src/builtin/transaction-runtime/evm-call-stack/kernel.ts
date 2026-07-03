// 本文件实现 EVM 外部调用、栈帧压入、返回值传播、revert 冒泡和深度保护内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { processRuntimeMessage, type RuntimeActor, type RuntimeMessage } from '../runtimeView';
import { callStackPhases, type CallStackState } from './model';
import { traceLinesForCallStack } from './trace';

/**
 * createInitialCallStackState 创建调用栈参与方。
 */
export function createInitialCallStackState(_params: SimInitParams, _seed: number): CallStackState {
  const actors: RuntimeActor[] = [{ id: 'eoa', label: '外部账户', role: 'runtime-actor', status: 'active' }, { id: 'a', label: '合约 A', role: 'runtime-actor', status: 'idle' }, { id: 'b', label: '合约 B', role: 'runtime-actor', status: 'idle' }];
  return finalizeCallStackState({ tick: 0, phase: callStackPhases[0].label, phaseIndex: 0, maxDepth: 4, frames: [], actors, messages: [], lastTransition: 'external', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceCallStackEvent 是 EVM 调用栈仿真的唯一事件入口。
 */
export function reduceCallStackEvent(state: CallStackState, event: SimEvent, _context: ReducerContext): CallStackState {
  if (event.type === 'select') return finalizeCallStackState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeCallStackState(revertDeep(state));
  if (event.type === 'recover') return finalizeCallStackState(popRecover(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeCallStackState(advanceCallStack(state, event));
  return state;
}

/**
 * advanceCallStack 按调用栈流程推进一个过程单元。
 */
export function advanceCallStack(state: CallStackState, event: SimEvent): CallStackState {
  const phaseIndex = Math.min(callStackPhases.length - 1, state.phaseIndex + 1);
  let next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: callStackPhases[phaseIndex].id };
  if (phaseIndex === 1) next = push(next, 'a', 'A');
  if (phaseIndex === 2) next = push(next, 'b', 'B');
  if (phaseIndex === 3) next = popRecover(next);
  if (phaseIndex === 4) next = { ...next, maxDepth: 3 };
  return next;
}

/**
 * finalizeCallStackState 刷新调用栈指标、检查点和代码追踪。
 */
export function finalizeCallStackState(state: CallStackState): CallStackState {
  const safe = state.frames.length <= state.maxDepth && !state.frames.some((frame) => frame.reverted);
  return { ...state, phase: callStackPhases[state.phaseIndex].label, actors: state.actors.map((actor) => ({ ...actor, status: actor.id === 'b' && state.frames.some((frame) => frame.reverted) ? 'danger' : actor.status })), explanation: explain(state.phaseIndex), metrics: { result: safe ? '调用栈可控' : '调用失败或过深', risk: safe ? 8 : 72, depth: state.frames.length }, checkpointValues: { safe }, _trace: { triggeredLines: traceLinesForCallStack(state.lastTransition), variables: { depth: state.frames.length }, executionPath: `call-stack/${state.lastTransition}` } };
}

/**
 * stackSafe 输出调用栈检查点。
 */
export function stackSafe(state: CallStackState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.safe), answer: { depth: state.frames.length, maxDepth: state.maxDepth }, explanation: state.checkpointValues.safe ? '调用深度和返回值均已受控。' : '仍存在 revert 或深度风险。' };
}

/**
 * push 压入一个调用栈帧。
 */
function push(state: CallStackState, contractId: string, name: string): CallStackState {
  const frame = { id: `frame-${state.frames.length + 1}`, contract: name, depth: state.frames.length + 1, returned: false, reverted: false };
  const from = state.frames.length === 0 ? 'eoa' : state.frames[state.frames.length - 1].contract.toLowerCase();
  return { ...state, frames: state.frames.concat(frame), messages: state.messages.concat(message(from, contractId, 'CALL', state.tick, '运行时压入新的合约调用栈帧。')) };
}

/**
 * revertDeep 让最深层调用失败。
 */
function revertDeep(state: CallStackState): CallStackState {
  return { ...state, phaseIndex: 3, lastTransition: 'revert', frames: state.frames.map((frame, index) => (index === state.frames.length - 1 ? { ...frame, reverted: true } : frame)) };
}

/**
 * popRecover 处理返回值并弹出成功栈帧。
 */
function popRecover(state: CallStackState): CallStackState {
  return state.frames.some((frame) => frame.reverted) ? { ...state, lastTransition: 'return', frames: state.frames.map((frame) => ({ ...frame, returned: false })) } : { ...state, lastTransition: 'return', frames: state.frames.map((frame) => ({ ...frame, returned: true })) };
}

/**
 * message 创建带过程跨度的调用消息。
 */
function message(from: string, to: string, label: string, at: number, detail: string): RuntimeMessage {
  return processRuntimeMessage({ id: deterministicId('stack-msg', { from, to, label, at }), from, to, label, at, status: 'delivered' }, detail);
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = callStackPhases[index] ?? callStackPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
