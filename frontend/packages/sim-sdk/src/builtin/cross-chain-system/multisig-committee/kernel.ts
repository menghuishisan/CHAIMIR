// 本文件实现委员会轮换、成员签名、门限聚合、恶意签名剔除和执行授权内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { aggregateCommitteeSignature, committeeMemberSignature, crossChainMessageHash } from '../crossChainPrimitives';
import { committeePhases, type CommitteeState } from './model';
import { traceLinesForCommittee } from './trace';

/**
 * createInitialCommitteeState 创建 3-of-5 跨链委员会。
 */
export function createInitialCommitteeState(_params: SimInitParams, _seed: number): CommitteeState {
  const messageHash = crossChainMessageHash('committee:chainA:chainB:v1', 21, 'mint-voucher-10');
  return finalizeCommitteeState({ tick: 0, phase: committeePhases[0].label, phaseIndex: 0, threshold: 3, messageHash, aggregateSignature: '', aggregateReady: false, authorized: false, members: ['A', 'B', 'C', 'D', 'E'].map((label) => ({ id: `member-${label}`, label: `成员 ${label}`, signed: false, malicious: false, active: true })), lastTransition: 'rotate', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceCommitteeEvent 是多签委员会仿真的唯一事件入口。
 */
export function reduceCommitteeEvent(state: CommitteeState, event: SimEvent, _context: ReducerContext): CommitteeState {
  if (event.type === 'attack') return finalizeCommitteeState(markMalicious(state));
  if (event.type === 'recover') return finalizeCommitteeState(filterMalicious(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeCommitteeState(advanceCommittee(state, event));
  return state;
}

/**
 * advanceCommittee 按多签授权流程推进。
 */
export function advanceCommittee(state: CommitteeState, event: SimEvent): CommitteeState {
  const phaseIndex = Math.min(committeePhases.length - 1, state.phaseIndex + 1);
  let next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: committeePhases[phaseIndex].id };
  if (phaseIndex === 1) next = signCommitteeMessage(next);
  if (phaseIndex === 2) next = aggregateCommittee(next);
  if (phaseIndex === 4) next = { ...next, authorized: next.aggregateReady && validSignatures(next) >= next.threshold };
  return next;
}

/**
 * finalizeCommitteeState 刷新委员会指标、检查点和代码追踪。
 */
export function finalizeCommitteeState(state: CommitteeState): CommitteeState {
  const valid = validSignatures(state);
  const authorized = state.authorized || (state.phaseIndex >= 4 && valid >= state.threshold);
  return { ...state, authorized, phase: committeePhases[state.phaseIndex].label, explanation: explain(state.phaseIndex), metrics: { result: authorized ? '授权通过' : '等待签名', risk: state.members.some((member) => member.malicious && member.signed) ? 76 : 8, validSignatures: valid }, checkpointValues: { authorized }, _trace: { triggeredLines: traceLinesForCommittee(state.lastTransition), variables: { validSignatures: valid, threshold: state.threshold, aggregateSignature: state.aggregateSignature }, executionPath: `multisig/${state.lastTransition}` } };
}

/**
 * committeeAuthorized 输出委员会授权检查点。
 */
export function committeeAuthorized(state: CommitteeState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.authorized), answer: { validSignatures: validSignatures(state), threshold: state.threshold }, explanation: state.checkpointValues.authorized ? '有效签名达到门限并完成授权。' : '有效签名不足或存在恶意签名。' };
}

/**
 * validSignatures 统计有效签名数量。
 */
export function validSignatures(state: CommitteeState): number {
  return state.members.filter((member) => member.active && member.signed && !member.malicious && member.signature === committeeMemberSignature(member.id, state.messageHash)).length;
}

/**
 * signCommitteeMessage 让前 threshold 个活跃成员对消息摘要签名。
 */
function signCommitteeMessage(state: CommitteeState): CommitteeState {
  let remaining = state.threshold;
  return {
    ...state,
    members: state.members.map((member) => {
      const signed = member.active && remaining > 0;
      if (signed) remaining -= 1;
      return { ...member, signed, signature: signed ? committeeMemberSignature(member.id, state.messageHash) : undefined };
    }),
  };
}

/**
 * aggregateCommittee 只聚合有效签名并生成授权摘要。
 */
function aggregateCommittee(state: CommitteeState): CommitteeState {
  const signatures = state.members.filter((member) => member.active && member.signed && !member.malicious && member.signature === committeeMemberSignature(member.id, state.messageHash)).map((member) => member.signature ?? '');
  return { ...state, aggregateReady: signatures.length >= state.threshold, aggregateSignature: signatures.length >= state.threshold ? aggregateCommitteeSignature(state.messageHash, signatures.slice(0, state.threshold)) : '' };
}

/**
 * markMalicious 注入恶意签名成员。
 */
function markMalicious(state: CommitteeState): CommitteeState {
  return { ...state, phaseIndex: 3, lastTransition: 'filter', members: state.members.map((member, index) => (index === 1 ? { ...member, malicious: true, signed: true, signature: committeeMemberSignature('forged-member', state.messageHash) } : member)), aggregateReady: false, aggregateSignature: '', authorized: false };
}

/**
 * filterMalicious 剔除恶意成员签名。
 */
function filterMalicious(state: CommitteeState): CommitteeState {
  const members = state.members.map((member) => (member.malicious ? { ...member, signed: false, active: false, signature: undefined } : member));
  return aggregateCommittee({ ...state, lastTransition: 'filter', members });
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = committeePhases[index] ?? committeePhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
