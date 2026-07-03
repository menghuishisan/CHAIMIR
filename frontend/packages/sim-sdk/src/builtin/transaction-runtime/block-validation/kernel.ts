// 本文件实现区块头、交易根、收据根、状态根校验和无效区块拒绝内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { blockHeaderDigest, blockValidationDigest, invalidValidationDigest } from '../runtimePrimitives';
import { blockValidationPhases, type BlockValidationState } from './model';
import { traceLinesForBlockValidation } from './trace';

/**
 * createInitialBlockValidationState 创建区块验证项。
 */
export function createInitialBlockValidationState(_params: SimInitParams, _seed: number): BlockValidationState {
  const items = ['父哈希', '交易根', '收据根', '状态根'].map((label) => {
    const hash = blockValidationDigest(label, 128);
    return { id: `validation-${label}`, label, expected: hash, actual: hash, valid: true };
  });
  return finalizeBlockValidationState({ tick: 0, phase: blockValidationPhases[0].label, phaseIndex: 0, blockHash: blockHeaderDigest(items), items, accepted: false, lastTransition: 'header', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceBlockValidationEvent 是区块验证仿真的唯一事件入口。
 */
export function reduceBlockValidationEvent(state: BlockValidationState, event: SimEvent, _context: ReducerContext): BlockValidationState {
  if (event.type === 'select') return finalizeBlockValidationState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeBlockValidationState(corruptStateRoot(state));
  if (event.type === 'recover') return finalizeBlockValidationState(recomputeRoots(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeBlockValidationState(advanceBlockValidation(state, event));
  return state;
}

/**
 * advanceBlockValidation 按区块验证流程推进。
 */
export function advanceBlockValidation(state: BlockValidationState, event: SimEvent): BlockValidationState {
  const phaseIndex = Math.min(blockValidationPhases.length - 1, state.phaseIndex + 1);
  return { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: blockValidationPhases[phaseIndex].id, accepted: phaseIndex >= 4 && state.items.every((item) => item.valid) };
}

/**
 * finalizeBlockValidationState 刷新区块验证指标、检查点和代码追踪。
 */
export function finalizeBlockValidationState(state: BlockValidationState): BlockValidationState {
  const accepted = state.accepted || (state.phaseIndex >= 4 && state.items.every((item) => item.valid));
  return { ...state, accepted, phase: blockValidationPhases[state.phaseIndex].label, explanation: explain(state.phaseIndex), metrics: { result: accepted ? '区块已接受' : state.items.some((item) => !item.valid) ? '区块被拒绝' : '验证中', risk: state.items.some((item) => !item.valid) ? 78 : 8 }, checkpointValues: { accepted }, _trace: { triggeredLines: traceLinesForBlockValidation(state.lastTransition), variables: { accepted }, executionPath: `block-validation/${state.lastTransition}` } };
}

/**
 * blockAccepted 输出区块验证检查点。
 */
export function blockAccepted(state: BlockValidationState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.accepted), answer: { accepted: state.accepted, invalid: state.items.filter((item) => !item.valid).map((item) => item.label) }, explanation: state.checkpointValues.accepted ? '区块所有验证项均通过。' : '区块仍有验证项未通过。' };
}

/**
 * corruptStateRoot 篡改状态根。
 */
function corruptStateRoot(state: BlockValidationState): BlockValidationState {
  return { ...state, phaseIndex: 3, lastTransition: 'state-root', accepted: false, items: state.items.map((item) => (item.label === '状态根' ? { ...item, actual: invalidValidationDigest(item.label), valid: false } : item)) };
}

/**
 * recomputeRoots 重算所有根摘要。
 */
function recomputeRoots(state: BlockValidationState): BlockValidationState {
  return { ...state, lastTransition: 'state-root', items: state.items.map((item) => ({ ...item, actual: item.expected, valid: true })) };
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = blockValidationPhases[index] ?? blockValidationPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
