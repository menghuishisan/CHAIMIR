// 本文件实现整数输入校验、溢出路径、checked 运算和边界用例覆盖内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { integerPhases, type IntegerBoundaryState } from './model';
import { traceLinesForInteger } from './trace';

/**
 * createInitialIntegerState 创建整数边界用例集合。
 */
export function createInitialIntegerState(_params: SimInitParams, _seed: number): IntegerBoundaryState {
  return finalizeIntegerState({ tick: 0, phase: integerPhases[0].label, phaseIndex: 0, maxValue: 1000, checkedMath: false, cappedInput: false, cases: [{ id: 'case-normal', label: '正常值', input: 20, result: 40, checked: false, failed: false }, { id: 'case-zero', label: '零值', input: 0, result: 0, checked: false, failed: false }, { id: 'case-max', label: '最大值', input: 1000, result: 2000, checked: false, failed: false }], lastTransition: 'input', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceIntegerEvent 是整数边界仿真的唯一事件入口。
 */
export function reduceIntegerEvent(state: IntegerBoundaryState, event: SimEvent, _context: ReducerContext): IntegerBoundaryState {
  if (event.type === 'select') return finalizeIntegerState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeIntegerState(overflow(state));
  if (event.type === 'recover') return finalizeIntegerState(enableChecked(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeIntegerState(advanceInteger(state, event));
  return state;
}

/**
 * advanceInteger 按输入、范围、计算、checked 和边界测试推进。
 */
export function advanceInteger(state: IntegerBoundaryState, event: SimEvent): IntegerBoundaryState {
  const phaseIndex = Math.min(integerPhases.length - 1, state.phaseIndex + 1);
  return { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: integerPhases[phaseIndex].id };
}

/**
 * finalizeIntegerState 刷新指标、检查点和代码追踪。
 */
export function finalizeIntegerState(state: IntegerBoundaryState): IntegerBoundaryState {
  const safe = state.checkedMath && state.cappedInput && state.cases.every((item) => item.input <= state.maxValue || item.failed);
  return { ...state, phase: integerPhases[state.phaseIndex].label, explanation: explain(state.phaseIndex), metrics: { result: safe ? '边界受控' : '存在边界风险', risk: safe ? 8 : 70, failedCases: state.cases.filter((item) => item.failed).length }, checkpointValues: { safe }, _trace: { triggeredLines: traceLinesForInteger(state.lastTransition), variables: { checkedMath: state.checkedMath, failedCases: state.cases.filter((item) => item.failed).length }, executionPath: `integer-boundary/${state.lastTransition}` } };
}

/**
 * integerSafe 输出整数边界检查点。
 */
export function integerSafe(state: IntegerBoundaryState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.safe), answer: { checkedMath: state.checkedMath, failedCases: state.metrics.failedCases }, explanation: state.checkpointValues.safe ? '极端输入已被 checked 运算和范围限制控制。' : '仍存在未受控整数边界。' };
}

/**
 * overflow 注入极大输入造成溢出风险。
 */
function overflow(state: IntegerBoundaryState): IntegerBoundaryState {
  return { ...state, phaseIndex: 2, lastTransition: 'compute', cases: state.cases.concat({ id: 'case-overflow', label: '极大值', input: 999999, result: 0, checked: state.checkedMath, failed: true }) };
}

/**
 * enableChecked 启用范围限制并拒绝超界用例。
 */
function enableChecked(state: IntegerBoundaryState): IntegerBoundaryState {
  return { ...state, phaseIndex: 3, lastTransition: 'checked', checkedMath: true, cappedInput: true, cases: state.cases.map((item) => ({ ...item, checked: true, failed: item.input > state.maxValue })) };
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = integerPhases[index] ?? integerPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
