// 本文件实现闪电贷借款、市场操纵、目标协议调用、还款和限额防护内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { integerParam } from '../../initParams';
import { processSecurityCall, type SecurityActor, type SecurityCall } from '../securityView';
import { flashLoanPhases, type FlashLoanState } from './model';
import { traceLinesForFlashLoan } from './trace';

/**
 * createInitialFlashLoanState 创建闪电贷组合攻击参与方。
 */
export function createInitialFlashLoanState(params: SimInitParams, _seed: number): FlashLoanState {
  const actors: SecurityActor[] = [{ id: 'flash-pool', label: '闪电贷池', role: 'security-actor', status: 'active' }, { id: 'amm', label: '交易池', role: 'security-actor', status: 'idle' }, { id: 'victim', label: '目标协议', role: 'security-actor', status: 'idle' }, { id: 'attacker', label: '攻击合约', role: 'security-actor', status: 'idle' }];
  const baseLoanAmount = integerParam(params, 'loanAmount', 1000, 1, 10_000_000);
  const basePoolPrice = integerParam(params, 'poolPrice', 100, 1, 1_000_000);
  return finalizeFlashLoanState({ tick: 0, phase: flashLoanPhases[0].label, phaseIndex: 0, baseLoanAmount, basePoolPrice, loanAmount: 0, poolPrice: basePoolPrice, protocolDebt: 0, attackerProfit: 0, limitEnabled: false, containedAttempt: false, actors, calls: [], lastTransition: 'borrow', explanation: explain(0), metrics: {}, checkpointValues: {} });
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
  if (phaseIndex === 1) next = { ...next, loanAmount: next.baseLoanAmount, calls: next.calls.concat(call('attacker', 'flash-pool', '借款', next.tick, '攻击合约在同一交易内借入大量资金。')) };
  if (phaseIndex === 2) next = { ...next, poolPrice: manipulatedPrice(next), calls: next.calls.concat(call('attacker', 'amm', '推价', next.tick, '借入资金短暂推偏交易池价格。')) };
  if (phaseIndex === 3) next = exploit(next);
  if (phaseIndex === 4) next = { ...next, calls: next.calls.concat(call('attacker', 'flash-pool', '还款', next.tick, '攻击在交易末尾归还本金和费用。')) };
  return next;
}

/**
 * finalizeFlashLoanState 刷新攻击收益、检查点和代码追踪。
 */
export function finalizeFlashLoanState(state: FlashLoanState): FlashLoanState {
  const safe = state.limitEnabled && state.containedAttempt && state.attackerProfit === 0 && state.protocolDebt === 0;
  return { ...state, phase: flashLoanPhases[state.phaseIndex].label, actors: state.actors.map((actor) => ({ ...actor, status: actor.id === 'attacker' && state.attackerProfit > 0 ? 'danger' : actor.id === 'victim' && safe ? 'success' : actor.status })), explanation: explain(state.phaseIndex), metrics: { result: safe ? '冲击受控' : state.attackerProfit > 0 ? '攻击获利' : '流程进行中', risk: safe ? 8 : state.attackerProfit > 0 ? 88 : 32, attackerProfit: state.attackerProfit }, checkpointValues: { contained: safe }, _trace: { triggeredLines: traceLinesForFlashLoan(state.lastTransition), variables: { attackerProfit: state.attackerProfit, poolPrice: state.poolPrice }, executionPath: `flash-loan/${state.lastTransition}` } };
}

/**
 * flashContained 输出闪电贷防护检查点。
 */
export function flashContained(state: FlashLoanState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.contained), answer: { profit: state.attackerProfit, limitEnabled: state.limitEnabled, containedAttempt: state.containedAttempt }, explanation: state.checkpointValues.contained ? '受保护交易已被限额和价格保护控制。' : '攻击仍可在单交易内获利。' };
}

/**
 * exploit 按异常状态从目标协议获利。
 */
function exploit(state: FlashLoanState): FlashLoanState {
  if (state.limitEnabled) return containFlashLoan(state);
  const loanAmount = Math.max(state.loanAmount, state.baseLoanAmount);
  return { ...state, phaseIndex: 3, lastTransition: 'exploit', loanAmount, poolPrice: manipulatedPrice(state), protocolDebt: Math.round(loanAmount * 0.42), attackerProfit: Math.round(loanAmount * 0.12), calls: state.calls.concat(call('attacker', 'victim', '异常借款', state.tick, '目标协议按被操纵状态错误放大借款额度。')) };
}

/**
 * enableLimit 启用单交易限额并执行受保护路径。
 */
function enableLimit(state: FlashLoanState): FlashLoanState {
  return containFlashLoan({ ...state, phaseIndex: 4, lastTransition: 'limit', limitEnabled: true, poolPrice: state.basePoolPrice, protocolDebt: 0, attackerProfit: 0 });
}

/**
 * containFlashLoan 记录限额、冷却时间和价格保护阻断组合攻击。
 */
function containFlashLoan(state: FlashLoanState): FlashLoanState {
  return { ...state, phaseIndex: 4, lastTransition: 'limit', poolPrice: state.basePoolPrice, protocolDebt: 0, attackerProfit: 0, containedAttempt: true, calls: state.calls.concat(call('attacker', 'victim', '受限调用', state.tick, '目标协议按单交易限额和价格保护拒绝异常放大。', 'dropped')) };
}

/**
 * manipulatedPrice 根据基准价格计算闪电贷冲击后的异常价格。
 */
function manipulatedPrice(state: FlashLoanState): number {
  return Math.round(state.basePoolPrice * 1.6);
}

/**
 * call 创建带过程跨度的协议调用。
 */
function call(from: string, to: string, label: string, at: number, detail: string, status: SecurityCall['status'] = 'delivered'): SecurityCall {
  return processSecurityCall({ id: deterministicId('flash-call', { from, to, label, at, status }), from, to, label, at, status }, detail);
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = flashLoanPhases[index] ?? flashLoanPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
