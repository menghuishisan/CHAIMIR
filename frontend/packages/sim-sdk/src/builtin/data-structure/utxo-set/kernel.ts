// 本文件实现 UTXO 输入引用、双花检测、金额守恒、找零输出和集合更新内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { traceLinesForUtxo } from './trace';
import { utxoPhases, type Utxo, type UtxoState } from './model';

/**
 * createInitialUtxoState 创建初始 UTXO 集合和待验证交易输入。
 */
export function createInitialUtxoState(_params: SimInitParams, _seed: number): UtxoState {
  const utxos = [utxo('u1', 'Alice', 8), utxo('u2', 'Alice', 5), utxo('u3', 'Bob', 4)];
  return finalizeUtxoState({ tick: 0, phase: utxoPhases[0].label, phaseIndex: 0, utxos, inputs: ['u1'], outputs: [], txValid: false, lastTransition: 'select', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceUtxoEvent 是 UTXO 集合仿真的唯一事件入口。
 */
export function reduceUtxoEvent(state: UtxoState, event: SimEvent, _context: ReducerContext): UtxoState {
  if (event.type === 'select') return finalizeUtxoState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeUtxoState(doubleSpend(state));
  if (event.type === 'recover') return finalizeUtxoState(compact(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeUtxoState(advanceUtxo(state, event));
  return state;
}

/**
 * advanceUtxo 按交易验证流程推进一个过程单元。
 */
export function advanceUtxo(state: UtxoState, event: SimEvent): UtxoState {
  const phaseIndex = Math.min(utxoPhases.length - 1, state.phaseIndex + 1);
  let next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: utxoPhases[phaseIndex].id };
  if (phaseIndex === 1) next = { ...next, utxos: next.utxos.map((item) => ({ ...item, selected: next.inputs.includes(item.id) })) };
  if (phaseIndex === 3) next = { ...next, outputs: [utxo('u4', 'Bob', 6), utxo('u5', 'Alice', 1)] };
  if (phaseIndex === 4) next = compact(next);
  return next;
}

/**
 * finalizeUtxoState 刷新指标、检查点和代码追踪。
 */
export function finalizeUtxoState(state: UtxoState): UtxoState {
  const inputSum = state.utxos.filter((item) => state.inputs.includes(item.id) && !item.spent).reduce((sum, item) => sum + item.amount, 0);
  const outputSum = state.outputs.reduce((sum, item) => sum + item.amount, 0);
  const hasDoubleSpend = hasDuplicateInput(state.inputs) || state.utxos.some((item) => item.doubleSpend);
  const valid = inputSum >= outputSum && !hasDoubleSpend;
  return { ...state, phase: utxoPhases[state.phaseIndex].label, explanation: explain(state.phaseIndex), metrics: { result: state.txValid ? '集合已更新' : valid ? '交易可执行' : '交易无效', risk: valid ? 8 : 78, inputSum, outputSum }, checkpointValues: { valid: state.txValid && valid }, _trace: { triggeredLines: traceLinesForUtxo(state.lastTransition), variables: { inputSum, outputSum, inputCount: state.inputs.length }, executionPath: `utxo/${state.lastTransition}` } };
}

/**
 * utxoValid 输出 UTXO 交易检查点。
 */
export function utxoValid(state: UtxoState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.valid);
  return { achieved, answer: { inputSum: state.metrics.inputSum, outputSum: state.metrics.outputSum }, explanation: achieved ? '输入未双花且 UTXO 集合已更新。' : '存在双花或集合尚未更新。' };
}

/**
 * doubleSpend 引用同一个输出两次以制造双花。
 */
function doubleSpend(state: UtxoState): UtxoState {
  return { ...state, phaseIndex: 1, lastTransition: 'check', inputs: ['u1', 'u1'], utxos: state.utxos.map((item) => (item.id === 'u1' ? { ...item, doubleSpend: true } : item)), txValid: false };
}

/**
 * compact 消费输入并加入找零输出。
 */
function compact(state: UtxoState): UtxoState {
  const hasDoubleSpend = hasDuplicateInput(state.inputs) || state.utxos.some((item) => item.doubleSpend);
  if (hasDoubleSpend) return { ...state, lastTransition: 'compact', txValid: false };
  const outputs = state.outputs.length > 0 ? state.outputs : [utxo('u4', 'Bob', 6), utxo('u5', 'Alice', 1)];
  return { ...state, lastTransition: 'compact', outputs, txValid: true, utxos: state.utxos.map((item) => (state.inputs.includes(item.id) ? { ...item, spent: true } : item)).concat(outputs) };
}

/**
 * hasDuplicateInput 检查同一 UTXO 是否被重复引用。
 */
function hasDuplicateInput(inputs: string[]): boolean {
  return new Set(inputs).size !== inputs.length;
}

/**
 * utxo 创建稳定 UTXO。
 */
function utxo(id: string, owner: string, amount: number): Utxo {
  return { id, owner, amount, spent: false, selected: false, doubleSpend: false };
}

/**
 * explain 生成当前阶段说明。
 */
function explain(index: number) {
  const phase = utxoPhases[index] ?? utxoPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
