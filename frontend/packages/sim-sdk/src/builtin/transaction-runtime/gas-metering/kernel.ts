// 本文件实现 gasLimit、逐指令扣费、out-of-gas、退款和费用结算内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { gasPhases, type GasState } from './model';
import { traceLinesForGas } from './trace';

/**
 * createInitialGasState 创建指令序列和初始 gasLimit。
 */
export function createInitialGasState(_params: SimInitParams, _seed: number): GasState {
  return finalizeGasState({ tick: 0, phase: gasPhases[0].label, phaseIndex: 0, gasLimit: 70, gasUsed: 0, refund: 0, outOfGas: false, steps: [{ op: 'SLOAD', cost: 20, executed: false, failed: false }, { op: 'CALL', cost: 30, executed: false, failed: false }, { op: 'SSTORE_CLEAR', cost: 15, executed: false, failed: false }], lastTransition: 'limit', explanation: explain(0), metrics: {}, checkpointValues: {} });
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
  const phaseIndex = Math.min(gasPhases.length - 1, state.phaseIndex + 1);
  const nextIndex = state.steps.findIndex((step) => !step.executed);
  if (nextIndex < 0) return { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, refund: 10, lastTransition: gasPhases[phaseIndex].id };
  const step = state.steps[nextIndex];
  const gasUsed = state.gasUsed + step.cost;
  const outOfGas = gasUsed > state.gasLimit;
  return { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, gasUsed, outOfGas, lastTransition: outOfGas ? 'oog' : gasPhases[phaseIndex].id, steps: state.steps.map((item, index) => (index === nextIndex ? { ...item, executed: !outOfGas, failed: outOfGas } : item)) };
}

/**
 * lowerLimit 降低 gasLimit 制造执行失败。
 */
function lowerLimit(state: GasState): GasState {
  return { ...state, phaseIndex: 2, lastTransition: 'oog', gasLimit: 35 };
}

/**
 * raiseLimit 提高 gasLimit 并清理失败状态。
 */
function raiseLimit(state: GasState): GasState {
  return { ...state, phaseIndex: 4, lastTransition: 'settle', gasLimit: 90, outOfGas: false, refund: 10, steps: state.steps.map((step) => ({ ...step, executed: true, failed: false })) };
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = gasPhases[index] ?? gasPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
