// 本文件实现源链锁定、消息构造、中继、目标链验证和执行回执内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { crossChainMessageHash } from '../crossChainPrimitives';
import { processCrossMessage, type CrossActor, type CrossMessage } from '../crossChainView';
import { crossMessagePhases, type CrossChainMessageState } from './model';
import { traceLinesForCrossMessage } from './trace';

/**
 * createInitialCrossMessageState 创建源链、中继和目标链。
 */
export function createInitialCrossMessageState(_params: SimInitParams, _seed: number): CrossChainMessageState {
  const actors: CrossActor[] = [{ id: 'source', label: '源链', role: 'cross-actor', status: 'active' }, { id: 'relayer', label: '中继者', role: 'cross-actor', status: 'idle' }, { id: 'target', label: '目标链', role: 'cross-actor', status: 'idle' }];
  return finalizeCrossMessageState({ tick: 0, phase: crossMessagePhases[0].label, phaseIndex: 0, messageId: crossChainMessageHash('chainA:chainB:v1', 17, 'transfer-asset-10'), locked: false, relayed: false, verified: false, executed: false, actors, messages: [], lastTransition: 'lock', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceCrossMessageEvent 是跨链消息生命周期仿真的唯一事件入口。
 */
export function reduceCrossMessageEvent(state: CrossChainMessageState, event: SimEvent, _context: ReducerContext): CrossChainMessageState {
  if (event.type === 'select') return finalizeCrossMessageState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeCrossMessageState(dropRelay(state));
  if (event.type === 'recover') return finalizeCrossMessageState(resubmit(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeCrossMessageState(advanceCrossMessage(state, event));
  return state;
}

/**
 * advanceCrossMessage 按跨链消息生命周期推进。
 */
export function advanceCrossMessage(state: CrossChainMessageState, event: SimEvent): CrossChainMessageState {
  const phaseIndex = Math.min(crossMessagePhases.length - 1, state.phaseIndex + 1);
  let next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: crossMessagePhases[phaseIndex].id };
  if (phaseIndex === 1) next = { ...next, locked: true, messages: next.messages.concat(message('source', 'relayer', '源链事件', next.tick, false, '源链锁定资产并产生可证明事件。')) };
  if (phaseIndex === 2) next = { ...next, relayed: true, messages: next.messages.concat(message('relayer', 'target', '提交证明', next.tick, false, '中继者提交消息和源链证明。')) };
  if (phaseIndex === 3) next = { ...next, verified: next.relayed };
  if (phaseIndex === 4) next = { ...next, executed: next.verified };
  return next;
}

/**
 * finalizeCrossMessageState 刷新跨链消息指标、检查点和代码追踪。
 */
export function finalizeCrossMessageState(state: CrossChainMessageState): CrossChainMessageState {
  const done = state.locked && state.relayed && state.verified && state.executed;
  return { ...state, phase: crossMessagePhases[state.phaseIndex].label, actors: state.actors.map((actor) => ({ ...actor, status: actor.id === 'target' && done ? 'success' : actor.status })), explanation: explain(state.phaseIndex), metrics: { result: done ? '消息已执行' : '等待跨链完成', risk: done ? 8 : state.relayed ? 20 : 55 }, checkpointValues: { done }, _trace: { triggeredLines: traceLinesForCrossMessage(state.lastTransition), variables: { messageId: state.messageId, executed: state.executed }, executionPath: `cross-message/${state.lastTransition}` } };
}

/**
 * messageExecuted 输出跨链消息检查点。
 */
export function messageExecuted(state: CrossChainMessageState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.done), answer: { messageId: state.messageId, executed: state.executed }, explanation: state.checkpointValues.done ? '跨链消息已验证并执行。' : '跨链消息尚未到达可执行终态。' };
}

/**
 * dropRelay 模拟中继丢失消息。
 */
function dropRelay(state: CrossChainMessageState): CrossChainMessageState {
  return { ...state, lastTransition: 'relay', relayed: false, verified: false, executed: false, messages: state.messages.concat(message('relayer', 'target', '中继丢失', state.tick, true, '中继者没有把证明提交到目标链。')) };
}

/**
 * resubmit 重新提交中继消息并完成验证。
 */
function resubmit(state: CrossChainMessageState): CrossChainMessageState {
  return { ...state, lastTransition: 'verify', relayed: true, verified: true, executed: true, messages: state.messages.concat(message('relayer', 'target', '重新提交', state.tick, false, '重新提交后目标链独立验证并执行。')) };
}

/**
 * message 创建带过程跨度的跨链消息。
 */
function message(from: string, to: string, label: string, at: number, dropped: boolean, detail: string): CrossMessage {
  return processCrossMessage({ id: deterministicId('cross-message', { from, to, label, at, dropped }), from, to, label, at, status: dropped ? 'dropped' : 'delivered' }, detail);
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = crossMessagePhases[index] ?? crossMessagePhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
