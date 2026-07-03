// 本文件实现 UTXO 输入引用、双花检测、金额守恒、找零输出和集合更新内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { integerArrayParam, integerParam, stringParam } from '../../initParams';
import { traceLinesForUtxo } from './trace';
import { utxoPhases, type Utxo, type UtxoState } from './model';

/**
 * createInitialUtxoState 创建初始 UTXO 集合和待验证交易输入。
 */
export function createInitialUtxoState(params: SimInitParams, _seed: number): UtxoState {
  const payer = stringParam(params, 'payer', 'Alice', 32);
  const recipient = stringParam(params, 'recipient', 'Bob', 32);
  const amounts = integerArrayParam(params, 'amounts', [8, 5, 4], 3, 12, 1, 10_000);
  const payAmount = integerParam(params, 'payAmount', 6, 1, Math.max(1, amounts[0] - 1));
  const fee = integerParam(params, 'fee', 1, 0, Math.max(0, amounts[0] - payAmount));
  const utxos = amounts.map((amount, index) => utxo(`u${index + 1}`, index < 2 ? payer : recipient, amount));
  return finalizeUtxoState({ tick: 0, phase: utxoPhases[0].label, phaseIndex: 0, utxos, inputs: ['u1'], outputs: [], recipient, payAmount, fee, txValid: false, lastTransition: 'select', explanation: explain(0), metrics: {}, checkpointValues: {} });
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
  if (phaseIndex === 3) next = { ...next, outputs: createOutputs(next) };
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
  const targetInput = state.inputs[0] ?? state.utxos.find((item) => !item.spent)?.id ?? 'u1';
  return { ...state, phaseIndex: 1, lastTransition: 'check', inputs: [targetInput, targetInput], utxos: state.utxos.map((item) => (item.id === targetInput ? { ...item, doubleSpend: true } : item)), txValid: false };
}

/**
 * compact 消费输入并加入找零输出。
 */
function compact(state: UtxoState): UtxoState {
  const hasDoubleSpend = hasDuplicateInput(state.inputs) || state.utxos.some((item) => item.doubleSpend);
  if (hasDoubleSpend) return { ...state, lastTransition: 'compact', txValid: false };
  const outputs = state.outputs.length > 0 ? state.outputs : createOutputs(state);
  return { ...state, lastTransition: 'compact', outputs, txValid: true, utxos: state.utxos.map((item) => (state.inputs.includes(item.id) ? { ...item, spent: true } : item)).concat(outputs) };
}

/**
 * createOutputs 按输入金额、支付金额和手续费生成收款与找零输出。
 */
function createOutputs(state: UtxoState): Utxo[] {
  const inputItems = state.utxos.filter((item) => state.inputs.includes(item.id) && !item.spent);
  const inputSum = inputItems.reduce((sum, item) => sum + item.amount, 0);
  const payer = inputItems[0]?.owner ?? state.utxos[0]?.owner ?? 'Alice';
  const payAmount = Math.min(state.payAmount, Math.max(1, inputSum - state.fee));
  const change = Math.max(0, inputSum - payAmount - state.fee);
  return [utxo('u-out-pay', state.recipient, payAmount)].concat(change > 0 ? [utxo('u-out-change', payer, change)] : []);
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
