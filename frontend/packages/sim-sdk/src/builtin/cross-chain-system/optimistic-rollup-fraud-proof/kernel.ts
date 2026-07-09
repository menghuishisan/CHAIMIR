// 本文件实现 Optimistic Rollup batch、挑战、二分和裁决内核。

import type { ChainBlock, CheckpointResult, ReducerContext, SimEvent, SimInitParams, TreeNode } from '../../../types';
import { optimisticRollupPhases, type DisputeSegment, type OptimisticRollupState } from './model';
import { traceLinesForOptimisticRollup } from './trace';

/** createInitialOptimisticRollupState 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function createInitialOptimisticRollupState(_params: SimInitParams, _seed: number): OptimisticRollupState {
  return finalizeOptimisticRollupState({
    tick: 0,
    phase: optimisticRollupPhases[0].label,
    phaseIndex: 0,
    l1Height: 900,
    batchId: 'batch-42',
    oldRoot: '0xold',
    claimedRoot: '0xclaim-bad',
    expectedRoot: '0xexpected',
    challengeWindow: 7,
    transactions: [
      { id: 'l2tx-1', action: 'deposit credit', valid: true },
      { id: 'l2tx-2', action: 'swap update', valid: false },
      { id: 'l2tx-3', action: 'withdraw debit', valid: true },
      { id: 'l2tx-4', action: 'fee account', valid: true },
    ],
    disputeSegments: [{ id: 'seg-0-3', fromStep: 0, toStep: 3, status: 'open' }],
    challenged: false,
    fraudProven: false,
    finalized: false,
    lastTransition: 'sequence',
    explanation: explain(0),
    metrics: {},
    checkpointValues: {},
  });
}

/** reduceOptimisticRollupEvent 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function reduceOptimisticRollupEvent(state: OptimisticRollupState, event: SimEvent, _context: ReducerContext): OptimisticRollupState {
  if (event.type === 'select') return finalizeOptimisticRollupState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeOptimisticRollupState({ ...state, phaseIndex: 2, challenged: true, claimedRoot: '0xclaim-bad', lastTransition: 'challenge' });
  if (event.type === 'recover') return finalizeOptimisticRollupState({ ...state, phaseIndex: 5, finalized: true, fraudProven: false, claimedRoot: state.expectedRoot, lastTransition: 'verdict' });
  if (event.type === 'advance' || event.type === 'tick') return finalizeOptimisticRollupState(advanceOptimisticRollup(state, event));
  return state;
}

/** finalizeOptimisticRollupState 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function finalizeOptimisticRollupState(state: OptimisticRollupState): OptimisticRollupState {
  return {
    ...state,
    phase: optimisticRollupPhases[state.phaseIndex].label,
    explanation: explain(state.phaseIndex),
    metrics: { result: state.fraudProven ? '欺诈成立并回滚' : state.finalized ? 'batch 最终确认' : state.challenged ? '挑战中' : '乐观等待', risk: state.fraudProven ? 70 : state.challenged ? 36 : 12, challengeWindow: state.challengeWindow },
    checkpointValues: { challenged: state.challenged, fraudProven: state.fraudProven, finalized: state.finalized, disputedStep: disputedStep(state) },
    _trace: {
      triggeredLines: traceLinesForOptimisticRollup(state.lastTransition),
      variables: { claimedRoot: state.claimedRoot, expectedRoot: state.expectedRoot, disputedStep: disputedStep(state) },
      executionPath: `optimistic-rollup/${state.lastTransition}`,
    },
  };
}

/** optimisticRollupCheckpoint 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function optimisticRollupCheckpoint(state: OptimisticRollupState): CheckpointResult {
  return {
    achieved: state.fraudProven || state.finalized,
    answer: { fraudProven: state.fraudProven, finalized: state.finalized, disputedStep: disputedStep(state) },
    explanation: state.fraudProven ? '二分定位到错误状态转换,L1 单步证明确认欺诈。' : state.finalized ? '挑战窗口内没有成立欺诈证明,batch 最终确认。' : '挑战流程尚未完成。',
  };
}

/** optimisticRollupChain 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function optimisticRollupChain(state: OptimisticRollupState): ChainBlock[] {
  return [
    { id: 'l1-prev', height: state.l1Height - 1, hash: 'l1-prev', parentHash: '', label: 'L1 previous', status: 'canonical' },
    { id: state.batchId, height: state.l1Height, hash: state.claimedRoot, parentHash: 'l1-prev', label: state.finalized ? 'finalized batch' : state.fraudProven ? 'reverted batch' : 'pending batch', status: state.fraudProven ? 'orphaned' : state.finalized ? 'canonical' : 'pending' },
  ];
}

/** disputeTree 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function disputeTree(state: OptimisticRollupState): TreeNode {
  const leaves = state.transactions.map((tx, index) => ({ id: tx.id, label: `${index}: ${tx.action}`, hash: tx.valid ? 'ok' : 'bad', meta: { id: tx.id, label: tx.action, lifecycle: { state: state.phaseIndex >= 3 ? 'active' as const : 'entering' as const, fromTick: Math.max(0, state.tick - 1) }, emphasis: tx.valid ? 'context' as const : 'focus' as const, explanation: tx.valid ? '状态转换匹配' : '争议状态转换' } }));
  return { id: 'trace-root', label: '执行 trace', hash: state.claimedRoot, children: [{ id: 'left-half', label: 'steps 0-1', hash: 'left', children: leaves.slice(0, 2) }, { id: 'right-half', label: 'steps 2-3', hash: 'right', children: leaves.slice(2) }] };
}

/** advanceOptimisticRollup 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function advanceOptimisticRollup(state: OptimisticRollupState, event: SimEvent): OptimisticRollupState {
  const tick = event.source === 'tick' ? state.tick + 1 : state.tick;
  if (state.phaseIndex === 0) return { ...state, tick, phaseIndex: 1, lastTransition: 'submit' };
  if (state.phaseIndex === 1) return { ...state, tick, phaseIndex: 2, challenged: true, lastTransition: 'challenge' };
  if (state.phaseIndex === 2) return { ...state, tick, phaseIndex: 3, lastTransition: 'bisect', disputeSegments: splitDispute(state.disputeSegments) };
  if (state.phaseIndex === 3) return { ...state, tick, phaseIndex: 4, lastTransition: 'prove', disputeSegments: state.disputeSegments.map((segment) => ({ ...segment, status: 'resolved' })) };
  if (state.phaseIndex === 4) return { ...state, tick, phaseIndex: 5, lastTransition: 'verdict', fraudProven: state.claimedRoot !== state.expectedRoot, finalized: state.claimedRoot === state.expectedRoot };
  return { ...state, tick, phaseIndex: 0, lastTransition: 'sequence' };
}

/** splitDispute 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function splitDispute(segments: DisputeSegment[]): DisputeSegment[] {
  const active = segments.find((segment) => segment.status === 'open') ?? segments[0];
  const mid = Math.floor((active.fromStep + active.toStep) / 2);
  return [{ ...active, status: 'split' }, { id: `seg-${active.fromStep}-${mid}`, fromStep: active.fromStep, toStep: mid, status: 'resolved' }, { id: `seg-${mid + 1}-${active.toStep}`, fromStep: mid + 1, toStep: active.toStep, status: 'open' }];
}

/** disputedStep 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function disputedStep(state: OptimisticRollupState): number {
  return state.transactions.findIndex((tx) => !tx.valid);
}

/** explain 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function explain(index: number) {
  const phase = optimisticRollupPhases[index] ?? optimisticRollupPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
