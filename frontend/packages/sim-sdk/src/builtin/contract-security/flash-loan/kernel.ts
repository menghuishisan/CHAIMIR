// 本文件实现闪电贷借款、市场操纵、目标协议调用、还款和限额防护内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { processSecurityCall, type SecurityActor, type SecurityCall } from '../securityView';
import { flashLoanPhases, type FlashLoanState } from './model';
import { traceLinesForFlashLoan } from './trace';

/**
 * createInitialFlashLoanState 创建闪电贷组合攻击参与方。
 */
export function createInitialFlashLoanState(_params: SimInitParams, _seed: number): FlashLoanState {
  const actors: SecurityActor[] = [{ id: 'flash-pool', label: '闪电贷池', role: 'security-actor', status: 'active' }, { id: 'amm', label: '交易池', role: 'security-actor', status: 'idle' }, { id: 'victim', label: '目标协议', role: 'security-actor', status: 'idle' }, { id: 'attacker', label: '攻击合约', role: 'security-actor', status: 'idle' }];
  return finalizeFlashLoanState({ tick: 0, phase: flashLoanPhases[0].label, phaseIndex: 0, loanAmount: 0, poolPrice: 100, protocolDebt: 0, attackerProfit: 0, limitEnabled: false, actors, calls: [], lastTransition: 'borrow', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceFlashLoanEvent 是闪电贷仿真的唯一事件入口。
 */
export function reduceFlashLoanEvent(state: FlashLoanState, event: SimEvent, _context: ReducerContext): FlashLoanState {
  if (event.type === 'select') return finalizeFlashLoanState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeFlashLoanState(exploit(state));
  if (event.type === 'recover') return finalizeFlashLoanState(enableLimit(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeFlashLoanState(advanceFlashLoan(state, event));
  return state;
}

/**
 * advanceFlashLoan 按原子交易步骤推进一个过程单元。
 */
export function advanceFlashLoan(state: FlashLoanState, event: SimEvent): FlashLoanState {
  const phaseIndex = Math.min(flashLoanPhases.length - 1, state.phaseIndex + 1);
  let next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: flashLoanPhases[phaseIndex].id };
  if (phaseIndex === 1) next = { ...next, loanAmount: 1000, calls: next.calls.concat(call('attacker', 'flash-pool', '借款', next.tick, '攻击合约在同一交易内借入大量资金。')) };
  if (phaseIndex === 2) next = { ...next, poolPrice: 160, calls: next.calls.concat(call('attacker', 'amm', '推价', next.tick, '借入资金短暂推偏交易池价格。')) };
  if (phaseIndex === 3) next = exploit(next);
  if (phaseIndex === 4) next = { ...next, calls: next.calls.concat(call('attacker', 'flash-pool', '还款', next.tick, '攻击在交易末尾归还本金和费用。')) };
  return next;
}

/**
 * finalizeFlashLoanState 刷新攻击收益、检查点和代码追踪。
 */
export function finalizeFlashLoanState(state: FlashLoanState): FlashLoanState {
  const safe = state.limitEnabled && state.attackerProfit === 0;
  return { ...state, phase: flashLoanPhases[state.phaseIndex].label, actors: state.actors.map((actor) => ({ ...actor, status: actor.id === 'attacker' && state.attackerProfit > 0 ? 'danger' : actor.id === 'victim' && safe ? 'success' : actor.status })), explanation: explain(state.phaseIndex), metrics: { result: safe ? '冲击受控' : state.attackerProfit > 0 ? '攻击获利' : '流程进行中', risk: safe ? 8 : state.attackerProfit > 0 ? 88 : 32, attackerProfit: state.attackerProfit }, checkpointValues: { contained: safe }, _trace: { triggeredLines: traceLinesForFlashLoan(state.lastTransition), variables: { attackerProfit: state.attackerProfit, poolPrice: state.poolPrice }, executionPath: `flash-loan/${state.lastTransition}` } };
}

/**
 * flashContained 输出闪电贷防护检查点。
 */
export function flashContained(state: FlashLoanState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.contained), answer: { profit: state.attackerProfit, limitEnabled: state.limitEnabled }, explanation: state.checkpointValues.contained ? '单交易冲击已被限额和价格保护控制。' : '攻击仍可在单交易内获利。' };
}

/**
 * exploit 按异常状态从目标协议获利。
 */
function exploit(state: FlashLoanState): FlashLoanState {
  if (state.limitEnabled) return state;
  return { ...state, phaseIndex: 3, lastTransition: 'exploit', loanAmount: Math.max(state.loanAmount, 1000), poolPrice: 160, protocolDebt: 420, attackerProfit: 120, calls: state.calls.concat(call('attacker', 'victim', '异常借款', state.tick, '目标协议按被操纵状态错误放大借款额度。')) };
}

/**
 * enableLimit 启用单交易限额并清除异常利润。
 */
function enableLimit(state: FlashLoanState): FlashLoanState {
  return { ...state, phaseIndex: 4, lastTransition: 'limit', limitEnabled: true, poolPrice: 101, protocolDebt: 0, attackerProfit: 0 };
}

/**
 * call 创建带过程跨度的协议调用。
 */
function call(from: string, to: string, label: string, at: number, detail: string): SecurityCall {
  return processSecurityCall({ id: deterministicId('flash-call', { from, to, label, at }), from, to, label, at, status: 'delivered' }, detail);
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = flashLoanPhases[index] ?? flashLoanPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
