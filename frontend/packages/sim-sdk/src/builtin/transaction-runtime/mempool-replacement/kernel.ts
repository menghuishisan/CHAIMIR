// 本文件实现 mempool nonce 队列、替换规则和节点视图传播内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { integerParam } from '../../initParams';
import { mempoolReplacementPhases, type MempoolReplacementState, type PoolTx } from './model';
import { traceLinesForMempoolReplacement } from './trace';

/** createInitialMempoolReplacementState 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function createInitialMempoolReplacementState(params: SimInitParams, _seed: number): MempoolReplacementState {
  const bump = integerParam(params, 'replacementBump', 10, 1, 100);
  return finalizeMempoolReplacementState({
    tick: 0,
    phase: mempoolReplacementPhases[0].label,
    phaseIndex: 0,
    expectedNonce: { Alice: 7, Bob: 2 },
    replacementRequiredBump: bump,
    transactions: [
      { id: 'alice-7-low', account: 'Alice', nonce: 7, fee: 30, status: 'pending', reason: '第一笔可执行 nonce' },
      { id: 'alice-8-next', account: 'Alice', nonce: 8, fee: 28, status: 'queued', reason: '等待 nonce 7 入块' },
      { id: 'bob-3-gap', account: 'Bob', nonce: 3, fee: 42, status: 'queued', reason: '缺少 Bob nonce 2' },
    ],
    nodeViews: [{ node: '本地节点', seen: ['alice-7-low', 'alice-8-next'] }, { node: '对等节点', seen: ['alice-7-low'] }, { node: '构建器', seen: [] }],
    messages: [],
    lastTransition: 'receive',
    explanation: explain(0),
    metrics: {},
    checkpointValues: {},
  });
}

/** reduceMempoolReplacementEvent 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function reduceMempoolReplacementEvent(state: MempoolReplacementState, event: SimEvent, _context: ReducerContext): MempoolReplacementState {
  if (event.type === 'select') return finalizeMempoolReplacementState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeMempoolReplacementState(tryWeakReplacement(state));
  if (event.type === 'recover') return finalizeMempoolReplacementState(tryValidReplacement(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeMempoolReplacementState(advanceMempool(state, event));
  return state;
}

/** finalizeMempoolReplacementState 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function finalizeMempoolReplacementState(state: MempoolReplacementState): MempoolReplacementState {
  const included = state.transactions.filter((tx) => tx.status === 'included').length;
  const rejected = state.transactions.filter((tx) => tx.status === 'rejected').length;
  return {
    ...state,
    phase: mempoolReplacementPhases[state.phaseIndex].label,
    explanation: explain(state.phaseIndex),
    metrics: { result: included > 0 ? '连续 nonce 已打包' : rejected > 0 ? '替换被拒绝' : '交易池排序中', risk: rejected > 0 ? 38 : 10, included, rejected },
    checkpointValues: {
      replacementAccepted: state.transactions.some((tx) => tx.id === 'alice-7-fast' && tx.status === 'pending'),
      queueReleased: state.transactions.some((tx) => tx.id === 'alice-8-next' && tx.status === 'pending'),
      rejectedWeakReplacement: rejected > 0,
    },
    _trace: {
      triggeredLines: traceLinesForMempoolReplacement(state.lastTransition),
      variables: { pending: state.transactions.filter((tx) => tx.status === 'pending').length, queued: state.transactions.filter((tx) => tx.status === 'queued').length },
      executionPath: `mempool/${state.lastTransition}`,
    },
  };
}

/** mempoolReplacementCheckpoint 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function mempoolReplacementCheckpoint(state: MempoolReplacementState): CheckpointResult {
  return {
    achieved: Boolean(state.checkpointValues.replacementAccepted || state.checkpointValues.queueReleased),
    answer: { replacementAccepted: state.checkpointValues.replacementAccepted, queueReleased: state.checkpointValues.queueReleased, rejectedWeakReplacement: state.checkpointValues.rejectedWeakReplacement },
    explanation: state.checkpointValues.replacementAccepted ? '加价交易满足替换阈值,旧交易被替换。' : '替换未满足阈值或队列仍被 nonce 缺口阻塞。',
  };
}

/** labelPoolActor 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function labelPoolActor(id: string): string {
  const labels: Record<string, string> = { user: '用户', local: '本地节点', peer: '对等节点', builder: '构建器', block: '区块' };
  return labels[id] ?? id;
}

/** advanceMempool 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function advanceMempool(state: MempoolReplacementState, event: SimEvent): MempoolReplacementState {
  const tick = event.source === 'tick' ? state.tick + 1 : state.tick;
  if (state.phaseIndex === 0) return { ...state, tick, phaseIndex: 1, lastTransition: 'queue', transactions: classifyByNonce(state) };
  if (state.phaseIndex === 1) return { ...state, tick, phaseIndex: 2, lastTransition: 'replace' };
  if (state.phaseIndex === 2) return { ...state, tick, phaseIndex: 3, lastTransition: 'propagate', messages: propagateMessages(tick) };
  if (state.phaseIndex === 3) return { ...state, tick, phaseIndex: 4, lastTransition: 'include', transactions: includePending(state) };
  if (state.phaseIndex === 4) return { ...state, tick, phaseIndex: 5, lastTransition: 'release', transactions: releaseQueued(state), expectedNonce: { ...state.expectedNonce, Alice: state.expectedNonce.Alice + 1 } };
  return { ...state, tick, phaseIndex: 1, lastTransition: 'queue', transactions: classifyByNonce(state) };
}

/** classifyByNonce 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function classifyByNonce(state: MempoolReplacementState): PoolTx[] {
  return state.transactions.map((tx) => {
    if (tx.status === 'included' || tx.status === 'replaced' || tx.status === 'rejected') return tx;
    const expected = state.expectedNonce[tx.account] ?? 0;
    if (tx.nonce === expected) return { ...tx, status: 'pending', reason: 'nonce 连续,可被构建器选择' };
    if (tx.nonce > expected) return { ...tx, status: 'queued', reason: `等待 nonce ${expected}` };
    return { ...tx, status: 'rejected', reason: 'nonce 过旧' };
  });
}

/** tryWeakReplacement 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function tryWeakReplacement(state: MempoolReplacementState): MempoolReplacementState {
  const base = state.transactions.find((tx) => tx.account === 'Alice' && tx.nonce === 7 && tx.status === 'pending');
  if (!base) return state;
  const weakFee = base.fee + Math.max(1, Math.floor((base.fee * (state.replacementRequiredBump - 3)) / 100));
  return {
    ...state,
    phaseIndex: 2,
    lastTransition: 'replace',
    transactions: [...state.transactions, { id: 'alice-7-weak', account: 'Alice', nonce: 7, fee: weakFee, status: 'rejected', reason: '加价不足,不能替换 pending 交易' }],
  };
}

/** tryValidReplacement 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function tryValidReplacement(state: MempoolReplacementState): MempoolReplacementState {
  const base = state.transactions.find((tx) => tx.account === 'Alice' && tx.nonce === 7 && tx.status === 'pending');
  if (!base) return state;
  const requiredFee = Math.ceil(base.fee * (1 + state.replacementRequiredBump / 100));
  return {
    ...state,
    phaseIndex: 2,
    lastTransition: 'replace',
    transactions: state.transactions.map((tx): PoolTx => (tx.id === base.id ? { ...tx, status: 'replaced', reason: '被更高报价同 nonce 交易替换' } : tx)).concat({ id: 'alice-7-fast', account: 'Alice', nonce: 7, fee: requiredFee, status: 'pending', reason: '满足替换加价阈值' }),
  };
}

/** propagateMessages 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function propagateMessages(tick: number) {
  return [
    { id: 'pool-msg-1', from: 'user', to: 'local', at: tick, endAt: tick + 1, label: 'submit tx', status: 'delivered' as const, detail: '用户提交交易到本地节点', process: { startedAt: tick, endedAt: tick + 1, progress: 1, label: '提交' } },
    { id: 'pool-msg-2', from: 'local', to: 'peer', at: tick + 1, endAt: tick + 3, label: 'gossip tx', status: 'sent' as const, detail: '交易池视图向对等节点传播', process: { startedAt: tick + 1, endedAt: tick + 3, progress: 0.6, label: '传播' } },
    { id: 'pool-msg-3', from: 'local', to: 'builder', at: tick + 2, endAt: tick + 4, label: 'candidate set', status: 'sent' as const, detail: '构建器接收候选 pending 集合', process: { startedAt: tick + 2, endedAt: tick + 4, progress: 0.5, label: '候选' } },
  ];
}

/** includePending 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function includePending(state: MempoolReplacementState): PoolTx[] {
  return state.transactions.map((tx) => (tx.account === 'Alice' && tx.nonce === state.expectedNonce.Alice && tx.status === 'pending' ? { ...tx, status: 'included', reason: '连续 nonce 被打包' } : tx));
}

/** releaseQueued 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function releaseQueued(state: MempoolReplacementState): PoolTx[] {
  const nextNonce = state.expectedNonce.Alice + 1;
  return state.transactions.map((tx) => (tx.account === 'Alice' && tx.nonce === nextNonce && tx.status === 'queued' ? { ...tx, status: 'pending', reason: '前序 nonce 入块后释放' } : tx));
}

/** explain 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function explain(index: number) {
  const phase = mempoolReplacementPhases[index] ?? mempoolReplacementPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
