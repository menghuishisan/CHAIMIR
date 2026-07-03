// 本文件实现交易构造、签名、交易池、区块打包、执行和回执内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { signedTransactionHash, transactionIntentHash } from '../runtimePrimitives';
import { processRuntimeMessage, type RuntimeActor, type RuntimeMessage } from '../runtimeView';
import { txLifecyclePhases, type TxLifecycleState } from './model';
import { traceLinesForTxLifecycle } from './trace';

/**
 * createInitialTxLifecycleState 创建交易生命周期参与方。
 */
export function createInitialTxLifecycleState(_params: SimInitParams, _seed: number): TxLifecycleState {
  const actors: RuntimeActor[] = [{ id: 'wallet', label: '钱包', role: 'runtime-actor', status: 'active' }, { id: 'node', label: '节点', role: 'runtime-actor', status: 'idle' }, { id: 'block', label: '区块', role: 'runtime-actor', status: 'idle' }, { id: 'vm', label: '执行器', role: 'runtime-actor', status: 'idle' }];
  const intentHash = transactionIntentHash('Alice', 'Bob', 10, 7);
  return finalizeTxLifecycleState({ tick: 0, phase: txLifecyclePhases[0].label, phaseIndex: 0, txHash: signedTransactionHash(intentHash, 'Alice'), signed: false, inMempool: false, included: false, executed: false, receipt: '', dropped: false, actors, messages: [], lastTransition: 'build', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceTxLifecycleEvent 是交易生命周期仿真的唯一事件入口。
 */
export function reduceTxLifecycleEvent(state: TxLifecycleState, event: SimEvent, _context: ReducerContext): TxLifecycleState {
  if (event.type === 'select') return finalizeTxLifecycleState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeTxLifecycleState(dropTx(state));
  if (event.type === 'recover') return finalizeTxLifecycleState(resubmit(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeTxLifecycleState(advanceTxLifecycle(state, event));
  return state;
}

/**
 * advanceTxLifecycle 按交易生命周期推进一个过程单元。
 */
export function advanceTxLifecycle(state: TxLifecycleState, event: SimEvent): TxLifecycleState {
  const phaseIndex = Math.min(txLifecyclePhases.length - 1, state.phaseIndex + 1);
  let next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: txLifecyclePhases[phaseIndex].id };
  if (phaseIndex === 1) next = { ...next, signed: true, messages: next.messages.concat(message('wallet', 'node', '提交签名交易', next.tick, '钱包签名后把交易广播给节点。')) };
  if (phaseIndex === 2 && !next.dropped) next = { ...next, inMempool: true };
  if (phaseIndex === 3 && next.inMempool) next = { ...next, included: true, messages: next.messages.concat(message('node', 'block', '打包交易', next.tick, '出块者从交易池选择交易写入区块。')) };
  if (phaseIndex === 4 && next.included) next = { ...next, executed: true, receipt: '成功', messages: next.messages.concat(message('block', 'vm', '执行交易', next.tick, '执行器产生状态转移和回执。')) };
  return next;
}

/**
 * finalizeTxLifecycleState 刷新指标、检查点和代码追踪。
 */
export function finalizeTxLifecycleState(state: TxLifecycleState): TxLifecycleState {
  return { ...state, phase: txLifecyclePhases[state.phaseIndex].label, actors: state.actors.map((actor) => ({ ...actor, status: actor.id === 'node' && state.dropped ? 'danger' : actor.id === 'vm' && state.executed ? 'success' : actor.status })), explanation: explain(state.phaseIndex), metrics: { result: state.receipt || '等待确认', risk: state.dropped ? 65 : state.executed ? 8 : 20 }, checkpointValues: { receipt: state.executed && state.receipt === '成功' }, _trace: { triggeredLines: traceLinesForTxLifecycle(state.lastTransition), variables: { txHash: state.txHash, receipt: state.receipt }, executionPath: `tx-lifecycle/${state.lastTransition}` } };
}

/**
 * receiptReady 输出回执检查点。
 */
export function receiptReady(state: TxLifecycleState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.receipt), answer: { receipt: state.receipt, txHash: state.txHash }, explanation: state.checkpointValues.receipt ? '交易已执行并生成成功回执。' : '交易尚未生成成功回执。' };
}

/**
 * dropTx 模拟交易被交易池丢弃。
 */
function dropTx(state: TxLifecycleState): TxLifecycleState {
  return { ...state, lastTransition: 'mempool', dropped: true, inMempool: false, receipt: '已丢弃' };
}

/**
 * resubmit 提高费用并重新提交交易。
 */
function resubmit(state: TxLifecycleState): TxLifecycleState {
  return { ...state, lastTransition: 'mempool', dropped: false, inMempool: true, receipt: '', messages: state.messages.concat(message('wallet', 'node', '提高费用重交', state.tick, '用户提高费用后重新进入交易池。')) };
}

/**
 * message 创建带过程跨度的运行时消息。
 */
function message(from: string, to: string, label: string, at: number, detail: string): RuntimeMessage {
  return processRuntimeMessage({ id: deterministicId('tx-life-msg', { from, to, label, at }), from, to, label, at, status: 'delivered' }, detail);
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = txLifecyclePhases[index] ?? txLifecyclePhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
