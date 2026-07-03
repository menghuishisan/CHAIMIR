// 本文件实现 gasLimit、逐指令扣费、out-of-gas、退款和费用结算内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { integerArrayParam, integerParam, stringArrayParam } from '../../initParams';
import { gasPhases, type GasState } from './model';
import { traceLinesForGas } from './trace';

/**
 * createInitialGasState 创建指令序列和初始 gasLimit。
 */
export function createInitialGasState(params: SimInitParams, _seed: number): GasState {
  const ops = stringArrayParam(params, 'ops', ['SLOAD', 'CALL', 'SSTORE_CLEAR'], 2, 12, 32);
  const costs = integerArrayParam(params, 'costs', [20, 30, 15], ops.length, ops.length, 1, 1_000_000);
  const defaultLimit = costs.reduce((sum, cost) => sum + cost, 0) + 5;
  return finalizeGasState({ tick: 0, phase: gasPhases[0].label, phaseIndex: 0, gasLimit: integerParam(params, 'gasLimit', defaultLimit, 1, 10_000_000), gasUsed: 0, refund: 0, outOfGas: false, steps: ops.map((op, index) => ({ op, cost: costs[index] ?? costs[0] ?? 1, executed: false, failed: false })), lastTransition: 'limit', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceGasEvent 是 Gas 计量仿真的唯一事件入口。
 */
export function reduceGasEvent(state: GasState, event: SimEvent, _context: ReducerContext): GasState {
  if (event.type === 'select') return finalizeGasState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeGasState(lowerLimit(state));
  if (event.type === 'recover') return finalizeGasState(raiseLimit(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeGasState(executeStep(state, event));
  return state;
}

/**
 * finalizeGasState 刷新 gas 指标、检查点和代码追踪。
 */
export function finalizeGasState(state: GasState): GasState {
  const settled = state.steps.every((step) => step.executed) && !state.outOfGas;
  return { ...state, phase: gasPhases[state.phaseIndex].label, explanation: explain(state.phaseIndex), metrics: { result: state.outOfGas ? 'Gas 不足回滚' : settled ? '费用已结算' : '执行中', risk: state.outOfGas ? 72 : 8, gasUsed: state.gasUsed }, checkpointValues: { settled }, _trace: { triggeredLines: traceLinesForGas(state.lastTransition), variables: { gasUsed: state.gasUsed, gasLimit: state.gasLimit }, executionPath: `gas/${state.lastTransition}` } };
}

/**
 * gasSettled 输出 gas 结算检查点。
 */
export function gasSettled(state: GasState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.settled), answer: { gasUsed: state.gasUsed, refund: state.refund }, explanation: state.checkpointValues.settled ? '所有指令执行完成且费用可结算。' : '执行尚未完成或已因 gas 不足回滚。' };
}

/**
 * executeStep 执行下一条指令并扣减 gas。
 */
function executeStep(state: GasState, event: SimEvent): GasState {
  const tick = event.source === 'tick' ? state.tick + 1 : state.tick;
  if (state.outOfGas) {
    return { ...state, phaseIndex: 2, tick, lastTransition: 'oog' };
  }
  const nextIndex = state.steps.findIndex((step) => !step.executed);
  if (nextIndex < 0 && state.phaseIndex < 3) {
    return { ...state, phaseIndex: 3, tick, refund: refundFor(state), lastTransition: 'refund' };
  }
  if (nextIndex < 0) {
    return { ...state, phaseIndex: 4, tick, lastTransition: 'settle' };
  }
  const step = state.steps[nextIndex];
  const gasUsed = state.gasUsed + step.cost;
  const outOfGas = gasUsed > state.gasLimit;
  return { ...state, phaseIndex: outOfGas ? 2 : 1, tick, gasUsed, outOfGas, lastTransition: outOfGas ? 'oog' : 'meter', steps: state.steps.map((item, index) => (index === nextIndex ? { ...item, executed: !outOfGas, failed: outOfGas } : item)) };
}

/**
 * lowerLimit 降低 gasLimit 制造执行失败。
 */
function lowerLimit(state: GasState): GasState {
  const executedCost = state.steps.filter((step) => step.executed).reduce((sum, step) => sum + step.cost, 0);
  const nextCost = state.steps.find((step) => !step.executed)?.cost ?? state.steps[state.steps.length - 1]?.cost ?? 1;
  return { ...state, phaseIndex: 2, lastTransition: 'oog', gasLimit: Math.max(1, executedCost + nextCost - 1) };
}

/**
 * raiseLimit 提高 gasLimit 并清理失败状态。
 */
function raiseLimit(state: GasState): GasState {
  const totalCost = state.steps.reduce((sum, step) => sum + step.cost, 0);
  const executedCost = state.steps.filter((step) => step.executed).reduce((sum, step) => sum + step.cost, 0);
  return { ...state, phaseIndex: 1, lastTransition: 'meter', gasLimit: Math.max(state.gasLimit, totalCost + refundFor(state)), gasUsed: executedCost, outOfGas: false, refund: 0, steps: state.steps.map((step) => ({ ...step, failed: false })) };
}

/**
 * refundFor 根据释放存储类指令计算有限退款。
 */
function refundFor(state: GasState): number {
  const refundable = state.steps.filter((step) => step.op.includes('CLEAR') || step.op.includes('REFUND')).reduce((sum, step) => sum + Math.floor(step.cost / 2), 0);
  return Math.min(Math.floor(state.gasUsed / 5), refundable);
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = gasPhases[index] ?? gasPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
