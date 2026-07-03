// 本文件实现账户 nonce 读取、缺口阻塞、替换交易和按序打包内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { noncePhases, type NonceState } from './model';
import { traceLinesForNonce } from './trace';

/**
 * createInitialNonceState 创建连续 nonce 交易。
 */
export function createInitialNonceState(_params: SimInitParams, _seed: number): NonceState {
  return finalizeNonceState({ tick: 0, phase: noncePhases[0].label, phaseIndex: 0, accountNonce: 5, txs: [{ id: 'tx-5', nonce: 5, fee: 10, status: 'pending' }, { id: 'tx-6', nonce: 6, fee: 9, status: 'pending' }, { id: 'tx-7', nonce: 7, fee: 8, status: 'pending' }], gapDetected: false, lastTransition: 'read', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceNonceEvent 是 Nonce 顺序仿真的唯一事件入口。
 */
export function reduceNonceEvent(state: NonceState, event: SimEvent, _context: ReducerContext): NonceState {
  if (event.type === 'select') return finalizeNonceState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeNonceState(createGap(state));
  if (event.type === 'recover') return finalizeNonceState(replaceNonce(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeNonceState(advanceNonce(state, event));
  return state;
}

/**
 * advanceNonce 按 nonce 排序流程推进。
 */
export function advanceNonce(state: NonceState, event: SimEvent): NonceState {
  const phaseIndex = Math.min(noncePhases.length - 1, state.phaseIndex + 1);
  const next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: noncePhases[phaseIndex].id };
  return phaseIndex === 4 && !state.gapDetected ? includeOrdered(next) : next;
}

/**
 * finalizeNonceState 刷新 nonce 指标、检查点和代码追踪。
 */
export function finalizeNonceState(state: NonceState): NonceState {
  const valid = !state.gapDetected && state.txs.every((tx) => tx.status !== 'blocked');
  return { ...state, phase: noncePhases[state.phaseIndex].label, explanation: explain(state.phaseIndex), metrics: { result: valid ? '顺序有效' : 'nonce 缺口阻塞', risk: valid ? 8 : 70, accountNonce: state.accountNonce }, checkpointValues: { valid: valid && state.accountNonce >= 8 }, _trace: { triggeredLines: traceLinesForNonce(state.lastTransition), variables: { accountNonce: state.accountNonce, gapDetected: state.gapDetected }, executionPath: `nonce/${state.lastTransition}` } };
}

/**
 * nonceValid 输出 nonce 顺序检查点。
 */
export function nonceValid(state: NonceState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.valid), answer: { accountNonce: state.accountNonce, gapDetected: state.gapDetected }, explanation: state.checkpointValues.valid ? '交易已按 nonce 顺序执行。' : '仍存在 nonce 缺口或尚未执行。' };
}

/**
 * createGap 删除前序交易造成阻塞。
 */
function createGap(state: NonceState): NonceState {
  return { ...state, phaseIndex: 2, lastTransition: 'gap', gapDetected: true, txs: state.txs.map((tx) => (tx.nonce >= 5 ? { ...tx, status: 'blocked' } : tx)) };
}

/**
 * replaceNonce 使用高费交易补齐 nonce 缺口。
 */
function replaceNonce(state: NonceState): NonceState {
  return { ...state, phaseIndex: 3, lastTransition: 'replace', gapDetected: false, txs: state.txs.map((tx) => (tx.nonce === 5 ? { id: 'tx-5b', nonce: 5, fee: 20, status: 'pending' } : { ...tx, status: 'pending' })) };
}

/**
 * includeOrdered 按顺序打包交易并推进账户 nonce。
 */
function includeOrdered(state: NonceState): NonceState {
  return { ...state, accountNonce: 8, txs: state.txs.map((tx) => ({ ...tx, status: 'included' })) };
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = noncePhases[index] ?? noncePhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
