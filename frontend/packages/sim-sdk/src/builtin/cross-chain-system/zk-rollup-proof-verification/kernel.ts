// 本文件实现 ZK Rollup batch、proof、verifier 和状态根更新内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { integerParam } from '../../initParams';
import { traceLinesForZkRollup } from './trace';
import { zkRollupPhases, type ZkRollupState } from './model';

/** createInitialZkRollupState 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function createInitialZkRollupState(params: SimInitParams, _seed: number): ZkRollupState {
  const batchSize = integerParam(params, 'batchSize', 128, 4, 4096);
  return finalizeZkRollupState({
    tick: 0,
    phase: zkRollupPhases[0].label,
    phaseIndex: 0,
    batchId: 'zk-batch-18',
    oldRoot: '0xold',
    newRoot: '0xnew',
    publicInputRoot: '0xnew',
    proofGenerated: false,
    proofValid: false,
    verifierAccepted: false,
    batchSize,
    provingTime: Math.max(3, Math.round(batchSize / 32)),
    inputs: [
      { id: 'tx-bundle', kind: 'tx', value: `${batchSize} tx`, valid: true },
      { id: 'old-root', kind: 'public-input', value: '0xold', valid: true },
      { id: 'new-root', kind: 'public-input', value: '0xnew', valid: true },
      { id: 'proof', kind: 'proof', value: 'waiting', valid: false },
    ],
    history: [{ x: 0, provingTime: Math.max(3, Math.round(batchSize / 32)), batchSize, l1Gas: 9 }],
    lastTransition: 'aggregate',
    explanation: explain(0),
    metrics: {},
    checkpointValues: {},
  });
}

/** reduceZkRollupEvent 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function reduceZkRollupEvent(state: ZkRollupState, event: SimEvent, _context: ReducerContext): ZkRollupState {
  if (event.type === 'select') return finalizeZkRollupState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeZkRollupState({ ...state, phaseIndex: 3, publicInputRoot: '0xwrong', proofGenerated: true, proofValid: false, verifierAccepted: false, lastTransition: 'verify', inputs: markProof({ ...state, publicInputRoot: '0xwrong' }, false) });
  if (event.type === 'recover') return finalizeZkRollupState({ ...state, phaseIndex: 3, publicInputRoot: state.newRoot, proofGenerated: true, proofValid: true, lastTransition: 'verify', inputs: markProof(state, true) });
  if (event.type === 'advance' || event.type === 'tick') return finalizeZkRollupState(advanceZkRollup(state, event));
  return state;
}

/** finalizeZkRollupState 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function finalizeZkRollupState(state: ZkRollupState): ZkRollupState {
  return {
    ...state,
    phase: zkRollupPhases[state.phaseIndex].label,
    explanation: explain(state.phaseIndex),
    metrics: { result: state.verifierAccepted ? '新状态根已接受' : state.phaseIndex === 5 ? '证明被拒绝' : '证明流程中', risk: state.phaseIndex === 5 ? 58 : state.verifierAccepted ? 4 : 16, batchSize: state.batchSize, provingTime: state.provingTime },
    checkpointValues: { proofValid: state.proofValid, verifierAccepted: state.verifierAccepted, rootMatches: state.publicInputRoot === state.newRoot },
    _trace: {
      triggeredLines: traceLinesForZkRollup(state.lastTransition),
      variables: { proofValid: state.proofValid, verifierAccepted: state.verifierAccepted, publicInputRoot: state.publicInputRoot },
      executionPath: `zk-rollup/${state.lastTransition}`,
    },
  };
}

/** zkRollupCheckpoint 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function zkRollupCheckpoint(state: ZkRollupState): CheckpointResult {
  return {
    achieved: state.verifierAccepted || state.phaseIndex === 5,
    answer: { proofValid: state.proofValid, verifierAccepted: state.verifierAccepted, rootMatches: state.publicInputRoot === state.newRoot },
    explanation: state.verifierAccepted ? 'proof 与 public inputs 匹配,L1 接受 newRoot。' : 'proof 或 public input 不匹配,L1 保持旧状态根。',
  };
}

/** advanceZkRollup 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function advanceZkRollup(state: ZkRollupState, event: SimEvent): ZkRollupState {
  const tick = event.source === 'tick' ? state.tick + 1 : state.tick;
  if (state.phaseIndex === 0) return { ...state, tick, phaseIndex: 1, lastTransition: 'trace' };
  if (state.phaseIndex === 1) return { ...state, tick, phaseIndex: 2, proofGenerated: true, provingTime: state.provingTime + 1, lastTransition: 'prove', inputs: markProof(state, false) };
  if (state.phaseIndex === 2) return { ...state, tick, phaseIndex: 3, proofValid: state.publicInputRoot === state.newRoot, lastTransition: 'verify', inputs: markProof(state, state.publicInputRoot === state.newRoot) };
  if (state.phaseIndex === 3) return state.proofValid ? { ...state, tick, phaseIndex: 4, verifierAccepted: true, lastTransition: 'update', history: [...state.history, { x: state.history.length, provingTime: state.provingTime, batchSize: state.batchSize, l1Gas: 10 }] } : { ...state, tick, phaseIndex: 5, lastTransition: 'reject' };
  return { ...state, tick, phaseIndex: 0, lastTransition: 'aggregate' };
}

/** markProof 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function markProof(state: ZkRollupState, valid: boolean) {
  return state.inputs.map((input) => (input.id === 'proof' ? { ...input, value: valid ? 'valid proof' : 'generated proof', valid } : input.id === 'new-root' ? { ...input, valid: state.publicInputRoot === state.newRoot } : input));
}

/** explain 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function explain(index: number) {
  const phase = zkRollupPhases[index] ?? zkRollupPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
