// 本文件实现源链确认数、最终性证明、重组风险检测和确认后释放内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { finalityPhases, type FinalityState } from './model';
import { traceLinesForFinality } from './trace';

/**
 * createInitialFinalityState 创建最终性确认状态。
 */
export function createInitialFinalityState(_params: SimInitParams, _seed: number): FinalityState {
  return finalizeFinalityState({ tick: 0, phase: finalityPhases[0].label, phaseIndex: 0, confirmations: 0, requiredConfirmations: 6, finalityProof: false, reorgDetected: false, released: false, lastTransition: 'observe', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceFinalityEvent 是最终性确认仿真的唯一事件入口。
 */
export function reduceFinalityEvent(state: FinalityState, event: SimEvent, _context: ReducerContext): FinalityState {
  if (event.type === 'attack') return finalizeFinalityState({ ...state, phaseIndex: 3, lastTransition: 'reorg', reorgDetected: true, released: false });
  if (event.type === 'recover') return finalizeFinalityState({ ...state, lastTransition: 'prove', reorgDetected: false, confirmations: state.requiredConfirmations, finalityProof: true });
  if (event.type === 'advance' || event.type === 'tick') return finalizeFinalityState(advanceFinality(state, event));
  return state;
}

/**
 * advanceFinality 增加确认数并推进最终性状态。
 */
export function advanceFinality(state: FinalityState, event: SimEvent): FinalityState {
  const phaseIndex = Math.min(finalityPhases.length - 1, state.phaseIndex + 1);
  const confirmations = Math.min(state.requiredConfirmations, state.confirmations + 2);
  const finalityProof = confirmations >= state.requiredConfirmations;
  return { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, confirmations, finalityProof, lastTransition: finalityPhases[phaseIndex].id, released: phaseIndex >= 4 && finalityProof && !state.reorgDetected };
}

/**
 * finalizeFinalityState 刷新最终性指标、检查点和代码追踪。
 */
export function finalizeFinalityState(state: FinalityState): FinalityState {
  const safe = state.released && state.finalityProof && !state.reorgDetected;
  return { ...state, phase: finalityPhases[state.phaseIndex].label, explanation: explain(state.phaseIndex), metrics: { result: safe ? '已安全释放' : state.reorgDetected ? '检测到重组' : '等待最终性', risk: safe ? 8 : state.reorgDetected ? 90 : 35 }, checkpointValues: { safe }, _trace: { triggeredLines: traceLinesForFinality(state.lastTransition), variables: { confirmations: state.confirmations, released: state.released }, executionPath: `finality/${state.lastTransition}` } };
}

/**
 * finalitySafe 输出最终性检查点。
 */
export function finalitySafe(state: FinalityState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.safe), answer: { confirmations: state.confirmations, released: state.released }, explanation: state.checkpointValues.safe ? '消息已在最终性确认后安全释放。' : '仍需等待最终性或处理重组风险。' };
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = finalityPhases[index] ?? finalityPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
