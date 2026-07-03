// 本文件实现跨链消息域分离、nonce、已执行集合、重放拒绝和版本轮换内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { replayProtectionHash } from '../crossChainPrimitives';
import { replayPhases, type ReplayState } from './model';
import { traceLinesForReplay } from './trace';

/**
 * createInitialReplayState 创建跨链消息重放防护状态。
 */
export function createInitialReplayState(_params: SimInitParams, _seed: number): ReplayState {
  const domain = 'chainA:chainB:v1';
  const nonce = 17;
  return finalizeReplayState({ tick: 0, phase: replayPhases[0].label, phaseIndex: 0, domain, nonce, messageHash: hash(domain, nonce), executedNonces: [], replayAttempt: false, accepted: false, lastTransition: 'domain', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceReplayEvent 是重放防护仿真的唯一事件入口。
 */
export function reduceReplayEvent(state: ReplayState, event: SimEvent, _context: ReducerContext): ReplayState {
  if (event.type === 'attack') return finalizeReplayState(replay(state));
  if (event.type === 'recover') return finalizeReplayState(rotate(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeReplayState(advanceReplay(state, event));
  return state;
}

/**
 * advanceReplay 按防重放流程推进。
 */
export function advanceReplay(state: ReplayState, event: SimEvent): ReplayState {
  const phaseIndex = Math.min(replayPhases.length - 1, state.phaseIndex + 1);
  let next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: replayPhases[phaseIndex].id };
  if (phaseIndex === 2) next = { ...next, accepted: true, executedNonces: Array.from(new Set(next.executedNonces.concat(next.nonce))) };
  if (phaseIndex === 3) next = replay(next);
  return next;
}

/**
 * finalizeReplayState 刷新重放防护指标、检查点和代码追踪。
 */
export function finalizeReplayState(state: ReplayState): ReplayState {
  const protectedNow = state.replayAttempt && state.executedNonces.includes(state.nonce) && !state.accepted;
  return { ...state, phase: replayPhases[state.phaseIndex].label, explanation: explain(state.phaseIndex), metrics: { result: protectedNow ? '重放已拒绝' : state.accepted ? '消息已执行' : '等待执行', risk: protectedNow ? 8 : state.accepted ? 15 : 35 }, checkpointValues: { protected: protectedNow }, _trace: { triggeredLines: traceLinesForReplay(state.lastTransition), variables: { nonce: state.nonce, domain: state.domain }, executionPath: `replay-protection/${state.lastTransition}` } };
}

/**
 * replayProtected 输出重放防护检查点。
 */
export function replayProtected(state: ReplayState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.protected), answer: { nonce: state.nonce, executedNonces: state.executedNonces }, explanation: state.checkpointValues.protected ? '重复消息已被已执行集合拒绝。' : '还未证明重放会被拒绝。' };
}

/**
 * replay 再次提交同 nonce 消息并拒绝。
 */
function replay(state: ReplayState): ReplayState {
  return { ...state, phaseIndex: 3, lastTransition: 'replay', replayAttempt: true, accepted: false };
}

/**
 * rotate 更新 domain 版本并保持旧 nonce 记录。
 */
function rotate(state: ReplayState): ReplayState {
  const domain = 'chainA:chainB:v2';
  return { ...state, phaseIndex: 4, lastTransition: 'rotate', domain, nonce: state.nonce + 1, messageHash: hash(domain, state.nonce + 1), replayAttempt: false, accepted: false };
}

/**
 * hash 生成跨链消息摘要。
 */
function hash(domain: string, nonce: number): string {
  return replayProtectionHash(domain, nonce);
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = replayPhases[index] ?? replayPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
