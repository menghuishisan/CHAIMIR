// 本文件实现锁仓证明、轻客户端同步、包含证明验证、目标链铸造和赎回闭环内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { bridgeProofHash, invalidBridgeProofHash } from '../crossChainPrimitives';
import { bridgePhases, type BridgeState } from './model';
import { traceLinesForBridge } from './trace';

/**
 * createInitialBridgeState 创建桥验证初始状态。
 */
export function createInitialBridgeState(_params: SimInitParams, _seed: number): BridgeState {
  return finalizeBridgeState({ tick: 0, phase: bridgePhases[0].label, phaseIndex: 0, proofHash: bridgeProofHash('chainA', 'chainB', 'lock-asset-10', 512), lightClientSynced: false, minted: false, redeemed: false, invalidProof: false, lastTransition: 'lock', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceBridgeEvent 是桥验证仿真的唯一事件入口。
 */
export function reduceBridgeEvent(state: BridgeState, event: SimEvent, _context: ReducerContext): BridgeState {
  if (event.type === 'attack') return finalizeBridgeState({ ...state, phaseIndex: 2, lastTransition: 'verify', proofHash: invalidBridgeProofHash(state.proofHash), invalidProof: true, minted: false });
  if (event.type === 'recover') return finalizeBridgeState({ ...state, lastTransition: 'sync', invalidProof: false, lightClientSynced: true });
  if (event.type === 'advance' || event.type === 'tick') return finalizeBridgeState(advanceBridge(state, event));
  return state;
}

/**
 * advanceBridge 按桥验证流程推进。
 */
export function advanceBridge(state: BridgeState, event: SimEvent): BridgeState {
  const phaseIndex = Math.min(bridgePhases.length - 1, state.phaseIndex + 1);
  let next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: bridgePhases[phaseIndex].id };
  if (phaseIndex === 1) next = { ...next, lightClientSynced: true };
  if (phaseIndex === 3 && next.lightClientSynced && !next.invalidProof) next = { ...next, minted: true };
  if (phaseIndex === 4 && next.minted) next = { ...next, redeemed: true };
  return next;
}

/**
 * finalizeBridgeState 刷新桥验证指标、检查点和代码追踪。
 */
export function finalizeBridgeState(state: BridgeState): BridgeState {
  const valid = state.lightClientSynced && !state.invalidProof && state.minted;
  return { ...state, phase: bridgePhases[state.phaseIndex].label, explanation: explain(state.phaseIndex), metrics: { result: valid ? '证明已验证' : state.invalidProof ? '证明被拒绝' : '等待验证', risk: valid ? 8 : state.invalidProof ? 82 : 30 }, checkpointValues: { valid }, _trace: { triggeredLines: traceLinesForBridge(state.lastTransition), variables: { proofHash: state.proofHash, minted: state.minted }, executionPath: `bridge-validation/${state.lastTransition}` } };
}

/**
 * bridgeValid 输出桥证明检查点。
 */
export function bridgeValid(state: BridgeState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.valid), answer: { proofHash: state.proofHash, minted: state.minted }, explanation: state.checkpointValues.valid ? '锁仓证明已验证并完成铸造。' : '桥证明尚未通过或已被拒绝。' };
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = bridgePhases[index] ?? bridgePhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
