// 本文件实现 EVM 外部调用、栈帧压入、返回值传播、revert 冒泡和深度保护内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { integerParam, stringArrayParam } from '../../initParams';
import { processRuntimeMessage, type RuntimeActor, type RuntimeMessage } from '../runtimeView';
import { callStackPhases, type CallStackState } from './model';
import { traceLinesForCallStack } from './trace';

/**
 * createInitialCallStackState 创建调用栈参与方。
 */
export function createInitialCallStackState(params: SimInitParams, _seed: number): CallStackState {
  const contracts = stringArrayParam(params, 'contracts', ['合约 A', '合约 B'], 2, 6, 32);
  const actors: RuntimeActor[] = [{ id: 'eoa', label: '外部账户', role: 'runtime-actor', status: 'active' }].concat(contracts.map((label, index) => ({ id: contractId(index), label, role: 'runtime-actor', status: 'idle' as const })));
  return finalizeCallStackState({ tick: 0, phase: callStackPhases[0].label, phaseIndex: 0, maxDepth: integerParam(params, 'maxDepth', 4, 2, 32), frames: [], actors, messages: [], lastTransition: 'external', explanation: explain(0), metrics: {}, checkpointValues: {} });
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
  if (state.phaseIndex === 2 && activeDepth(state) > 0) {
    return popRecover({ ...state, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: 'return' });
  }
  const pendingContractIndex = Math.min(state.frames.length, state.actors.length - 2);
  if (state.phaseIndex === 1 && pendingContractIndex > 0 && state.frames.length < state.actors.length - 1 && state.frames.length < state.maxDepth) {
    return push({ ...state, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: 'push' }, contractId(pendingContractIndex), contractLabel(state, pendingContractIndex));
  }
  const phaseIndex = Math.min(callStackPhases.length - 1, state.phaseIndex + 1);
  let next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: callStackPhases[phaseIndex].id };
  if (phaseIndex === 1) next = push(next, contractId(0), contractLabel(next, 0));
  if (phaseIndex === 2) next = popRecover(next);
  if (phaseIndex === 4) next = { ...next, maxDepth: Math.max(1, next.maxDepth - 1) };
  return next;
}

/**
 * finalizeCallStackState 刷新调用栈指标、检查点和代码追踪。
 */
export function finalizeCallStackState(state: CallStackState): CallStackState {
  const depth = activeDepth(state);
  const safe = depth <= state.maxDepth && !state.frames.some((frame) => frame.reverted);
  return { ...state, phase: callStackPhases[state.phaseIndex].label, actors: state.actors.map((actor) => ({ ...actor, status: state.frames.some((frame) => frame.reverted && actor.label === frame.contract) ? 'danger' : actor.status })), explanation: explain(state.phaseIndex), metrics: { result: safe ? '调用栈可控' : '调用失败或过深', risk: safe ? 8 : 72, depth }, checkpointValues: { safe }, _trace: { triggeredLines: traceLinesForCallStack(state.lastTransition), variables: { depth }, executionPath: `call-stack/${state.lastTransition}` } };
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
  const from = state.frames.length === 0 ? 'eoa' : contractIdForDepth(state.frames.length);
  return { ...state, frames: state.frames.concat(frame), messages: state.messages.concat(message(from, contractId, 'CALL', state.tick, '运行时压入新的合约调用栈帧。')) };
}

/**
 * contractId 生成调用参与方的稳定 ID。
 */
function contractId(index: number): string {
  return `contract-${index + 1}`;
}

/**
 * contractIdForDepth 根据当前栈深度定位上一层合约参与方。
 */
function contractIdForDepth(depth: number): string {
  return contractId(Math.max(0, depth - 1));
}

/**
 * contractLabel 返回指定合约标签。
 */
function contractLabel(state: CallStackState, index: number): string {
  return state.actors.find((actor) => actor.id === contractId(index))?.label ?? `合约 ${index + 1}`;
}

/**
 * revertDeep 让最深层调用失败。
 */
function revertDeep(state: CallStackState): CallStackState {
  const activeIndex = findDeepestActiveFrameIndex(state);
  const targetIndex = activeIndex >= 0 ? activeIndex : state.frames.length - 1;
  return { ...state, phaseIndex: 3, lastTransition: 'revert', frames: state.frames.map((frame, index) => (index === targetIndex ? { ...frame, reverted: true, returned: false } : frame)) };
}

/**
 * popRecover 处理最深活跃栈帧的返回值。
 */
function popRecover(state: CallStackState): CallStackState {
  if (state.frames.some((frame) => frame.reverted)) {
    return { ...state, lastTransition: 'return', frames: state.frames.map((frame) => ({ ...frame, returned: false })) };
  }
  const targetIndex = findDeepestActiveFrameIndex(state);
  return { ...state, lastTransition: 'return', frames: state.frames.map((frame, index) => (index === targetIndex ? { ...frame, returned: true } : frame)) };
}

/**
 * activeDepth 统计尚未返回的活跃调用帧数量。
 */
function activeDepth(state: CallStackState): number {
  return state.frames.filter((frame) => !frame.returned).length;
}

/**
 * findDeepestActiveFrameIndex 找到当前最深的未返回栈帧。
 */
function findDeepestActiveFrameIndex(state: CallStackState): number {
  for (let index = state.frames.length - 1; index >= 0; index -= 1) {
    if (!state.frames[index].returned) return index;
  }
  return -1;
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
