// 本文件实现门限签名密钥分片、部分签名、聚合、验证和故障恢复内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import type { CryptoMessage } from '../cryptoView';
import { aggregateThresholdSignature, groupMul, messageDigest, partialThresholdSignature, polynomialShare, roundDigest } from '../cryptoPrimitives';
import { thresholdSignaturePhases, type ShareHolder, type ThresholdState } from './model';
import { traceLinesForThresholdSignature } from './trace';

/**
 * createInitialThresholdSignatureState 创建 3-of-5 门限签名场景。
 */
export function createInitialThresholdSignatureState(_params: SimInitParams, _seed: number): ThresholdState {
  const secret = 23;
  const polynomial = [7, 11];
  const digest = messageDigest('threshold-signature', 'committee release', 1);
  const holders = ['A', 'B', 'C', 'D', 'E'].map<ShareHolder>((label, index) => {
    const x = index + 1;
    const shareValue = polynomialShare(secret, polynomial, x);
    return { id: `share-${x}`, label: `签名者 ${label}`, role: 'share-holder', status: 'idle', value: `x=${x}`, share: roundDigest('threshold-share', `${x}:${shareValue}`, 10), x, shareValue, signed: false, faulty: false };
  });
  return finalizeThresholdSignatureState({ tick: 0, phase: thresholdSignaturePhases[0].label, phaseIndex: 0, threshold: 3, messageDigest: digest, groupPublicKey: groupMul(secret), polynomial, aggregateSignature: '', holders, messages: [], aggregateValid: false, lastTransition: 'split', explanation: explainThresholdPhase(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceThresholdSignatureEvent 是门限签名包唯一事件入口。
 */
export function reduceThresholdSignatureEvent(state: ThresholdState, event: SimEvent, _context: ReducerContext): ThresholdState {
  if (event.type === 'select') return finalizeThresholdSignatureState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeThresholdSignatureState(markFaultyShare(state));
  if (event.type === 'recover') return finalizeThresholdSignatureState(replaceFaultyShare(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeThresholdSignatureState(advanceThresholdSignature(state, event));
  return state;
}

/**
 * advanceThresholdSignature 推进门限签名阶段。
 */
export function advanceThresholdSignature(state: ThresholdState, event: SimEvent): ThresholdState {
  const phaseIndex = Math.min(thresholdSignaturePhases.length - 1, state.phaseIndex + 1);
  const next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: thresholdSignaturePhases[phaseIndex].id };
  if (phaseIndex === 2) return partialSign(next);
  if (phaseIndex === 3) return aggregate(next);
  if (phaseIndex >= 4) return verifyAggregate(next);
  return next;
}

/**
 * aggregateValid 输出门限签名检查点。
 */
export function thresholdAggregateValid(state: ThresholdState): CheckpointResult {
  return { achieved: state.aggregateValid, answer: { validShares: validShares(state), threshold: state.threshold }, explanation: state.aggregateValid ? '有效份额达到门限,聚合签名可验证。' : '有效份额不足,不能形成聚合签名。' };
}

/**
 * finalizeThresholdSignatureState 刷新门限签名派生状态。
 */
export function finalizeThresholdSignatureState(state: ThresholdState): ThresholdState {
  const valid = validShares(state);
  return {
    ...state,
    phase: thresholdSignaturePhases[state.phaseIndex].label,
    holders: state.holders.map((holder) => ({ ...holder, status: holder.faulty ? 'danger' : holder.signed ? 'success' : 'idle' })),
    explanation: explainThresholdPhase(state.phaseIndex),
    metrics: { result: state.aggregateValid ? '聚合签名有效' : '等待足够份额', risk: valid < state.threshold ? 70 : 8, validShares: valid },
    checkpointValues: { aggregateValid: state.aggregateValid, validShares: valid },
    _trace: { triggeredLines: traceLinesForThresholdSignature(state.lastTransition), variables: { threshold: state.threshold, validShares: valid, aggregateSignature: state.aggregateSignature }, executionPath: `threshold/${state.lastTransition}` },
  };
}

/**
 * partialSign 让前 t 个非故障签名者生成部分签名。
 */
function partialSign(state: ThresholdState): ThresholdState {
  let remaining = state.threshold;
  const holders = state.holders.map((holder) => {
    const signed = !holder.faulty && remaining > 0;
    if (signed) remaining -= 1;
    return { ...holder, signed, partialSignature: signed ? partialSignature(state.messageDigest, holder.x, holder.shareValue) : undefined };
  });
  return { ...state, holders, messages: state.messages.concat(holders.filter((holder) => holder.signed).map((holder) => message(holder.id, 'aggregator', '部分签名', state.tick))) };
}

/**
 * aggregate 收集有效份额并生成聚合签名。
 */
function aggregate(state: ThresholdState): ThresholdState {
  const parts = state.holders
    .filter((holder) => holder.signed && !holder.faulty && holder.partialSignature === partialSignature(state.messageDigest, holder.x, holder.shareValue))
    .map((holder) => ({ x: holder.x, partial: holder.partialSignature ?? '' }));
  return { ...state, aggregateSignature: validShares(state) >= state.threshold ? aggregateThresholdSignature(state.messageDigest, parts.slice(0, state.threshold)) : '' };
}

/**
 * verifyAggregate 用群公钥语义校验聚合签名。
 */
function verifyAggregate(state: ThresholdState): ThresholdState {
  const parts = state.holders
    .filter((holder) => holder.signed && !holder.faulty && holder.partialSignature === partialSignature(state.messageDigest, holder.x, holder.shareValue))
    .map((holder) => ({ x: holder.x, partial: holder.partialSignature ?? '' }))
    .slice(0, state.threshold);
  const expectedAggregate = parts.length >= state.threshold ? aggregateThresholdSignature(state.messageDigest, parts) : '';
  return { ...state, aggregateValid: state.groupPublicKey > 0 && state.aggregateSignature === expectedAggregate };
}

/**
 * markFaultyShare 标记一个已签名份额为故障。
 */
function markFaultyShare(state: ThresholdState): ThresholdState {
  return { ...state, phaseIndex: 5, lastTransition: 'exclude', aggregateValid: false, holders: state.holders.map((holder, index) => (index === 1 ? { ...holder, faulty: true, signed: false, partialSignature: roundDigest('faulty-threshold-share', holder.share, 12) } : holder)) };
}

/**
 * replaceFaultyShare 剔除故障份额并补足候补签名者。
 */
function replaceFaultyShare(state: ThresholdState): ThresholdState {
  let need = state.threshold - validShares(state);
  const holders = state.holders.map((holder) => {
    if (!holder.faulty && !holder.signed && need > 0) {
      need -= 1;
      return { ...holder, signed: true, partialSignature: partialSignature(state.messageDigest, holder.x, holder.shareValue) };
    }
    return holder;
  });
  return verifyAggregate(aggregate({ ...state, phaseIndex: 5, lastTransition: 'exclude', holders }));
}

/**
 * validShares 统计有效部分签名。
 */
export function validShares(state: ThresholdState): number {
  return state.holders.filter((holder) => holder.signed && !holder.faulty && holder.partialSignature === partialSignature(state.messageDigest, holder.x, holder.shareValue)).length;
}

/**
 * partialSignature 生成可验证的教学用部分签名。
 */
function partialSignature(digest: string, x: number, shareValue: number): string {
  return partialThresholdSignature(digest, x, shareValue);
}

/**
 * message 创建份额提交消息。
 */
function message(from: string, to: string, label: string, at: number): CryptoMessage {
  return { id: deterministicId('share-msg', { from, to, label, at }), from, to, label, at, status: 'delivered' };
}

/**
 * explainThresholdPhase 生成阶段说明。
 */
function explainThresholdPhase(index: number) {
  const phase = thresholdSignaturePhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
