// 本文件实现 EIP-1559 交易选择、费用拆分和 base fee 调整内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { integerParam } from '../../initParams';
import { feeMarketPhases, type FeeMarketState, type FeeMarketTx } from './model';
import { traceLinesForFeeMarket } from './trace';

/** createInitialFeeMarketState 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function createInitialFeeMarketState(params: SimInitParams, seed: number): FeeMarketState {
  const baseFee = integerParam(params, 'baseFee', 30 + (Math.abs(seed) % 8), 1, 500);
  const targetGas = integerParam(params, 'targetGas', 90_000, 21_000, 1_000_000);
  return finalizeFeeMarketState({
    tick: 0,
    phase: feeMarketPhases[0].label,
    phaseIndex: 0,
    blockNumber: 1,
    baseFee,
    targetGas,
    gasUsed: 0,
    nextBaseFee: baseFee,
    congested: false,
    transactions: seedTransactions(baseFee),
    history: [{ x: 0, baseFee, gasUsed: 0, tip: 0 }],
    lastTransition: 'quote',
    explanation: explain(0),
    metrics: {},
    checkpointValues: {},
  });
}

/** reduceFeeMarketEvent 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function reduceFeeMarketEvent(state: FeeMarketState, event: SimEvent, _context: ReducerContext): FeeMarketState {
  if (event.type === 'select') return finalizeFeeMarketState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeFeeMarketState(addCongestion(state));
  if (event.type === 'recover') return finalizeFeeMarketState(addPatientBid(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeFeeMarketState(advanceFeeMarket(state, event));
  return state;
}

/** finalizeFeeMarketState 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function finalizeFeeMarketState(state: FeeMarketState): FeeMarketState {
  const included = state.transactions.filter((tx) => tx.included);
  const accepted = included.length > 0 && state.phaseIndex >= 4;
  return {
    ...state,
    phase: feeMarketPhases[state.phaseIndex].label,
    explanation: explain(state.phaseIndex),
    metrics: {
      result: accepted ? '费用已拆分' : state.congested ? '拥堵报价中' : '等待选择',
      risk: state.congested ? 42 : 12,
      baseFee: state.baseFee,
      nextBaseFee: state.nextBaseFee,
      gasUsed: state.gasUsed,
    },
    checkpointValues: {
      baseFeeRises: state.nextBaseFee > state.baseFee,
      includedCount: included.length,
      feeSplitDone: included.length > 0 && state.phaseIndex >= 4 && included.every((tx) => tx.burned > 0),
    },
    _trace: {
      triggeredLines: traceLinesForFeeMarket(state.lastTransition),
      variables: { baseFee: state.baseFee, gasUsed: state.gasUsed, nextBaseFee: state.nextBaseFee },
      executionPath: `eip1559/${state.lastTransition}`,
    },
  };
}

/** feeMarketCheckpoint 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
export function feeMarketCheckpoint(state: FeeMarketState): CheckpointResult {
  const rises = Boolean(state.checkpointValues.baseFeeRises);
  return {
    achieved: Boolean(state.checkpointValues.feeSplitDone),
    answer: { baseFeeRises: rises, includedCount: state.checkpointValues.includedCount, nextBaseFee: state.nextBaseFee },
    explanation: rises ? '当前区块 gasUsed 高于 targetGas,下一块 base fee 会上升。' : '当前区块未超过目标 gas,base fee 不会上升。',
  };
}

/** advanceFeeMarket 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function advanceFeeMarket(state: FeeMarketState, event: SimEvent): FeeMarketState {
  const tick = event.source === 'tick' ? state.tick + 1 : state.tick;
  if (state.phaseIndex === 0) return { ...state, tick, phaseIndex: 1, lastTransition: 'select', transactions: selectTransactions(state) };
  if (state.phaseIndex === 1) return { ...state, tick, phaseIndex: 2, lastTransition: 'execute', gasUsed: selectedGas(state.transactions) };
  if (state.phaseIndex === 2) return { ...state, tick, phaseIndex: 3, lastTransition: 'split', transactions: splitFees(state) };
  if (state.phaseIndex === 3) return adjustBaseFee({ ...state, tick, phaseIndex: 4, lastTransition: 'adjust' });
  if (state.phaseIndex === 4) return enterNextBlock({ ...state, tick, phaseIndex: 5, lastTransition: 'settle' });
  return { ...state, tick, phaseIndex: 0, lastTransition: 'quote', transactions: resetWaiting(state.transactions), gasUsed: 0 };
}

/** selectTransactions 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function selectTransactions(state: FeeMarketState): FeeMarketTx[] {
  let capacity = state.targetGas * 2;
  return [...state.transactions]
    .sort((a, b) => effectiveTip(b, state.baseFee) - effectiveTip(a, state.baseFee))
    .map((tx) => {
      const valid = tx.maxFeePerGas >= state.baseFee && capacity >= tx.gasLimit;
      if (valid) capacity -= tx.gasLimit;
      return { ...tx, included: valid, dropped: tx.maxFeePerGas < state.baseFee };
    })
    .sort((a, b) => a.id.localeCompare(b.id));
}

/** splitFees 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function splitFees(state: FeeMarketState): FeeMarketTx[] {
  return state.transactions.map((tx) => {
    if (!tx.included) return tx;
    const tipPerGas = effectiveTip(tx, state.baseFee);
    const paid = tx.gasLimit * (state.baseFee + tipPerGas);
    return { ...tx, burned: tx.gasLimit * state.baseFee, tip: tx.gasLimit * tipPerGas, paid, refunded: Math.max(0, tx.gasLimit * tx.maxFeePerGas - paid) };
  });
}

/** adjustBaseFee 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function adjustBaseFee(state: FeeMarketState): FeeMarketState {
  const delta = state.gasUsed - state.targetGas;
  const change = Math.trunc((state.baseFee * delta) / state.targetGas / 8);
  const nextBaseFee = Math.max(1, state.baseFee + change);
  return { ...state, nextBaseFee, history: [...state.history, { x: state.blockNumber, baseFee: state.baseFee, gasUsed: state.gasUsed, tip: totalTip(state.transactions) }] };
}

/** enterNextBlock 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function enterNextBlock(state: FeeMarketState): FeeMarketState {
  return { ...state, blockNumber: state.blockNumber + 1, baseFee: state.nextBaseFee };
}

/** addCongestion 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function addCongestion(state: FeeMarketState): FeeMarketState {
  const id = `tx${state.transactions.length + 1}`;
  return { ...state, congested: true, phaseIndex: 0, lastTransition: 'quote', transactions: [...state.transactions, { id, sender: '套利者', gasLimit: state.targetGas, maxFeePerGas: state.baseFee + 80, maxPriorityFeePerGas: 12, included: false, dropped: false, paid: 0, burned: 0, tip: 0, refunded: 0 }] };
}

/** addPatientBid 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function addPatientBid(state: FeeMarketState): FeeMarketState {
  return { ...state, congested: false, phaseIndex: 0, lastTransition: 'quote', transactions: state.transactions.map((tx) => (tx.dropped ? { ...tx, maxFeePerGas: state.baseFee + 15, dropped: false } : tx)) };
}

/** seedTransactions 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function seedTransactions(baseFee: number): FeeMarketTx[] {
  return [
    { id: 'tx1', sender: 'Alice', gasLimit: 21_000, maxFeePerGas: baseFee + 8, maxPriorityFeePerGas: 2, included: false, dropped: false, paid: 0, burned: 0, tip: 0, refunded: 0 },
    { id: 'tx2', sender: 'Bob', gasLimit: 45_000, maxFeePerGas: baseFee - 2, maxPriorityFeePerGas: 5, included: false, dropped: false, paid: 0, burned: 0, tip: 0, refunded: 0 },
    { id: 'tx3', sender: 'Carol', gasLimit: 70_000, maxFeePerGas: baseFee + 40, maxPriorityFeePerGas: 9, included: false, dropped: false, paid: 0, burned: 0, tip: 0, refunded: 0 },
  ];
}

/** resetWaiting 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function resetWaiting(txs: FeeMarketTx[]): FeeMarketTx[] {
  return txs.filter((tx) => !tx.included).map((tx) => ({ ...tx, included: false, dropped: false, paid: 0, burned: 0, tip: 0, refunded: 0 }));
}

/** effectiveTip 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function effectiveTip(tx: FeeMarketTx, baseFee: number): number {
  return Math.max(0, Math.min(tx.maxPriorityFeePerGas, tx.maxFeePerGas - baseFee));
}

/** selectedGas 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function selectedGas(txs: FeeMarketTx[]): number {
  return txs.filter((tx) => tx.included).reduce((sum, tx) => sum + tx.gasLimit, 0);
}

/** totalTip 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function totalTip(txs: FeeMarketTx[]): number {
  return txs.reduce((sum, tx) => sum + tx.tip, 0);
}

/** explain 执行当前内置仿真的状态推进、事件计算或校验逻辑。 */
function explain(index: number) {
  const phase = feeMarketPhases[index] ?? feeMarketPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
