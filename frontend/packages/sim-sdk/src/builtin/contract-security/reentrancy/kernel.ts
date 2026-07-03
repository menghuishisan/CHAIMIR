// 本文件实现重入攻击的提款、外部调用、fallback 重入和重入锁防护内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { integerParam } from '../../initParams';
import { processSecurityCall, type SecurityActor, type SecurityCall } from '../securityView';
import { reentrancyPhases, type ReentrancyState } from './model';
import { traceLinesForReentrancy } from './trace';

/**
 * createInitialReentrancyState 创建金库、普通用户和攻击合约。
 */
export function createInitialReentrancyState(params: SimInitParams, _seed: number): ReentrancyState {
  const actors: SecurityActor[] = [{ id: 'user', label: '普通用户', role: 'security-actor', status: 'idle' }, { id: 'vault', label: '金库合约', role: 'security-actor', status: 'active', value: '持有资金' }, { id: 'attacker', label: '攻击合约', role: 'security-actor', status: 'idle', value: '可回调' }];
  const vaultBalance = integerParam(params, 'vaultBalance', 100, 1, 1_000_000);
  const attackerCredit = integerParam(params, 'attackerCredit', 10, 1, vaultBalance);
  return finalizeReentrancyState({ tick: 0, phase: reentrancyPhases[0].label, phaseIndex: 0, vaultBalance, attackerCredit, attackerBalance: 0, lockEnabled: false, reentered: false, blockedReentry: false, actors, calls: [], lastTransition: 'deposit', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceReentrancyEvent 是重入仿真的唯一事件入口。
 */
export function reduceReentrancyEvent(state: ReentrancyState, event: SimEvent, _context: ReducerContext): ReentrancyState {
  if (event.type === 'select') return finalizeReentrancyState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeReentrancyState(reenter(state));
  if (event.type === 'recover') return finalizeReentrancyState(guard(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeReentrancyState(advanceReentrancy(state, event));
  return state;
}

/**
 * advanceReentrancy 按重入攻击调用栈推进一个过程单元。
 */
export function advanceReentrancy(state: ReentrancyState, event: SimEvent): ReentrancyState {
  const phaseIndex = Math.min(reentrancyPhases.length - 1, state.phaseIndex + 1);
  const next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: reentrancyPhases[phaseIndex].id };
  if (phaseIndex === 1) return { ...next, calls: next.calls.concat(call('attacker', 'vault', 'withdraw', next.tick, '攻击合约发起合法提款入口。')) };
  if (phaseIndex === 2) return { ...next, vaultBalance: next.vaultBalance - next.attackerCredit, attackerBalance: next.attackerBalance + next.attackerCredit, calls: next.calls.concat(call('vault', 'attacker', '转账', next.tick, '外部转账早于余额扣减。')) };
  if (phaseIndex === 3) return reenter(next);
  if (phaseIndex === 4) return guard(next);
  return next;
}

/**
 * finalizeReentrancyState 刷新参与方状态、指标、检查点和代码追踪。
 */
export function finalizeReentrancyState(state: ReentrancyState): ReentrancyState {
  const safe = state.lockEnabled && state.blockedReentry;
  return { ...state, phase: reentrancyPhases[state.phaseIndex].label, actors: state.actors.map((actor) => ({ ...actor, status: actor.id === 'attacker' && state.reentered && !safe ? 'danger' : actor.id === 'vault' && safe ? 'success' : actor.status })), explanation: explain(state.phaseIndex), metrics: { result: safe ? '重入已阻断' : state.reentered ? '重入发生' : '流程进行中', risk: state.reentered && !safe ? 90 : safe ? 8 : 30, vaultBalance: state.vaultBalance }, checkpointValues: { blocked: safe }, _trace: { triggeredLines: traceLinesForReentrancy(state.lastTransition), variables: { vaultBalance: state.vaultBalance, attackerBalance: state.attackerBalance }, executionPath: `reentrancy/${state.lastTransition}` } };
}

/**
 * reentrancyBlocked 输出重入防护检查点。
 */
export function reentrancyBlocked(state: ReentrancyState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.blocked), answer: { lockEnabled: state.lockEnabled, blockedReentry: state.blockedReentry }, explanation: state.checkpointValues.blocked ? '受保护提款已阻断 fallback 重入。' : '提款流程仍可被重入。' };
}

/**
 * reenter 在余额扣减前再次提款。
 */
function reenter(state: ReentrancyState): ReentrancyState {
  if (state.lockEnabled) return blockReentry(state);
  return { ...state, phaseIndex: 3, lastTransition: 'callback', reentered: true, vaultBalance: state.vaultBalance - state.attackerCredit, attackerBalance: state.attackerBalance + state.attackerCredit, calls: state.calls.concat(call('attacker', 'vault', 'fallback 重入', state.tick, 'fallback 在状态未更新时再次进入提款。')) };
}

/**
 * guard 启用重入锁并执行一次受保护提款,验证 fallback 重入被拒绝。
 */
function guard(state: ReentrancyState): ReentrancyState {
  return blockReentry({ ...state, phaseIndex: 4, lastTransition: 'guard', lockEnabled: true });
}

/**
 * blockReentry 记录受保护路径中的 fallback 调用被重入锁拒绝。
 */
function blockReentry(state: ReentrancyState): ReentrancyState {
  return { ...state, phaseIndex: 4, lastTransition: 'guard', blockedReentry: true, calls: state.calls.concat(call('attacker', 'vault', 'fallback 被拒绝', state.tick, '重入锁在回调再次进入时拒绝提款。', 'dropped')) };
}

/**
 * call 创建带过程跨度的合约调用消息。
 */
function call(from: string, to: string, label: string, at: number, detail: string, status: SecurityCall['status'] = 'delivered'): SecurityCall {
  return processSecurityCall({ id: deterministicId('reentrancy-call', { from, to, label, at, status }), from, to, label, at, status }, detail);
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = reentrancyPhases[index] ?? reentrancyPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
