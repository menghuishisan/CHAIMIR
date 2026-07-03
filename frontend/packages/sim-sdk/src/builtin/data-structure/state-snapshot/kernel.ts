// 本文件实现状态快照的同高收集、根摘要、脏写记录、回滚和一致性校验内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { integerArrayParam, stringArrayParam } from '../../initParams';
import { stateRootHash } from '../dataPrimitives';
import { snapshotPhases, type AccountState, type SnapshotState } from './model';
import { traceLinesForSnapshot } from './trace';

/**
 * createInitialSnapshotState 创建账户状态和初始快照根。
 */
export function createInitialSnapshotState(params: SimInitParams, _seed: number): SnapshotState {
  const accounts = baseAccounts(params);
  const root = computeStateRoot(accounts);
  return finalizeSnapshotState({ tick: 0, phase: snapshotPhases[0].label, phaseIndex: 0, accounts, snapshotAccounts: accounts, snapshotRoot: root, currentRoot: root, rollbackRoot: root, samples: [{ x: 0, consistency: 100, risk: 8, cost: 20 }], lastTransition: 'collect', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceSnapshotEvent 是状态快照仿真的唯一事件入口。
 */
export function reduceSnapshotEvent(state: SnapshotState, event: SimEvent, _context: ReducerContext): SnapshotState {
  if (event.type === 'select') return finalizeSnapshotState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeSnapshotState(applyDirtyWrite(state));
  if (event.type === 'recover') return finalizeSnapshotState(rollbackSnapshot(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeSnapshotState(advanceSnapshot(state, event));
  return state;
}

/**
 * advanceSnapshot 按快照与回滚流程推进一个过程单元。
 */
export function advanceSnapshot(state: SnapshotState, event: SimEvent): SnapshotState {
  const phaseIndex = Math.min(snapshotPhases.length - 1, state.phaseIndex + 1);
  let next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: snapshotPhases[phaseIndex].id };
  if (phaseIndex === 2) next = applyDirtyWrite(next);
  if (phaseIndex === 3) next = rollbackSnapshot(next);
  return next;
}

/**
 * finalizeSnapshotState 刷新趋势、检查点和代码追踪。
 */
export function finalizeSnapshotState(state: SnapshotState): SnapshotState {
  const consistent = state.currentRoot === state.snapshotRoot;
  const samples = state.samples.concat({ x: state.tick + state.phaseIndex, consistency: consistent ? 100 : 45, risk: consistent ? 8 : 72, cost: 25 + state.accounts.filter((account) => account.dirty).length * 12 }).slice(-24);
  return { ...state, phase: snapshotPhases[state.phaseIndex].label, samples, explanation: explain(state.phaseIndex), metrics: { result: consistent ? '快照一致' : '状态偏离快照', risk: consistent ? 8 : 72, dirty: state.accounts.filter((account) => account.dirty).length }, checkpointValues: { consistent }, _trace: { triggeredLines: traceLinesForSnapshot(state.lastTransition), variables: { snapshotRoot: state.snapshotRoot, currentRoot: state.currentRoot }, executionPath: `snapshot/${state.lastTransition}` } };
}

/**
 * snapshotValid 输出快照根一致性检查点。
 */
export function snapshotValid(state: SnapshotState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.consistent);
  return { achieved, answer: { snapshotRoot: state.snapshotRoot, currentRoot: state.currentRoot }, explanation: achieved ? '当前根与快照根一致。' : '当前根仍偏离快照根。' };
}

/**
 * computeStateRoot 计算账户状态根。
 */
export function computeStateRoot(accounts: AccountState[]): string {
  return stateRootHash(accounts);
}

/**
 * applyDirtyWrite 模拟执行过程中的脏写入。
 */
function applyDirtyWrite(state: SnapshotState): SnapshotState {
  const target = state.selectedElementId ?? state.accounts[1]?.id ?? state.accounts[0]?.id;
  const accounts = state.accounts.map((account) => (account.id === target ? { ...account, balance: account.balance + 9, nonce: account.nonce + 1, dirty: true } : account));
  return { ...state, phaseIndex: Math.max(state.phaseIndex, 2), lastTransition: 'delta', accounts, currentRoot: computeStateRoot(accounts) };
}

/**
 * rollbackSnapshot 恢复到快照版本。
 */
function rollbackSnapshot(state: SnapshotState): SnapshotState {
  return { ...state, phaseIndex: Math.max(state.phaseIndex, 3), lastTransition: 'rollback', accounts: state.snapshotAccounts.map((account) => ({ ...account, restored: true })), currentRoot: state.snapshotRoot, rollbackRoot: state.snapshotRoot };
}

/**
 * baseAccounts 返回快照基线账户。
 */
function baseAccounts(params: SimInitParams): AccountState[] {
  const ids = stringArrayParam(params, 'accounts', ['Alice', 'Bob', 'Carol'], 2, 12, 32);
  const balances = integerArrayParam(params, 'balances', [30, 20, 12], ids.length, ids.length, 0, 1_000_000);
  const nonces = integerArrayParam(params, 'nonces', [1, 0, 4], ids.length, ids.length, 0, 1_000_000);
  return ids.map((id, index) => ({ id, balance: balances[index] ?? 0, nonce: nonces[index] ?? 0, dirty: false, restored: false }));
}

/**
 * explain 生成当前阶段说明。
 */
function explain(index: number) {
  const phase = snapshotPhases[index] ?? snapshotPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
