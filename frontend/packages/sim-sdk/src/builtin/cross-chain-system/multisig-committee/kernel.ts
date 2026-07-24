// 本文件实现委员会轮换、成员签名、门限聚合、恶意签名剔除和执行授权内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { integerParam, stringArrayParam, stringParam } from '../../initParams';
import { aggregateCommitteeSignature, committeeMemberSignature, crossChainMessageHash } from '../crossChainPrimitives';
import { committeePhases, type CommitteeMember, type CommitteeState } from './model';
import { traceLinesForCommittee } from './trace';

/**
 * createInitialCommitteeState 根据参数创建跨链委员会。
 */
export function createInitialCommitteeState(params: SimInitParams, _seed: number): CommitteeState {
  const labels = stringArrayParam(params, 'members', ['A', 'B', 'C', 'D', 'E'], 3, 21, 24);
  const threshold = integerParam(params, 'threshold', 3, 2, labels.length);
  const domain = stringParam(params, 'domain', 'committee:chainA:chainB:v1', 64);
  const nonce = integerParam(params, 'nonce', 21, 0, 1_000_000);
  const payload = stringParam(params, 'payload', 'mint-voucher-10', 96);
  const messageHash = crossChainMessageHash(domain, nonce, payload);
  return finalizeCommitteeState({ tick: 0, phase: committeePhases[0].label, phaseIndex: 0, threshold, messageHash, aggregateSignature: '', aggregateReady: false, authorized: false, members: labels.map((label, index) => ({ id: `member-${index + 1}`, label: `成员 ${label}`, signed: false, malicious: false, active: true })), lastTransition: 'rotate', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceCommitteeEvent 是多签委员会仿真的唯一事件入口。
 */
export function reduceCommitteeEvent(state: CommitteeState, event: SimEvent, _context: ReducerContext): CommitteeState {
  if (event.type === 'select') return finalizeCommitteeState({ ...state, selectedElementId: event.target });
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
  return state.members.filter((member) => isValidMemberSignature(state, member)).length;
}

/** isValidMemberSignature 校验活跃委员会成员对当前消息摘要的签名。 */
function isValidMemberSignature(state: CommitteeState, member: CommitteeMember): boolean {
  return member.active && member.signed && !member.malicious && member.signature === committeeMemberSignature(member.id, state.messageHash);
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
  const signatures = state.members.filter((member) => isValidMemberSignature(state, member)).map((member) => member.signature ?? '');
  return { ...state, aggregateReady: signatures.length >= state.threshold, aggregateSignature: signatures.length >= state.threshold ? aggregateCommitteeSignature(state.messageHash, signatures.slice(0, state.threshold)) : '' };
}

/**
 * markMalicious 注入恶意签名成员。
 */
function markMalicious(state: CommitteeState): CommitteeState {
  const targetId = state.selectedElementId ?? state.members.find((member) => member.signed)?.id ?? state.members[0]?.id;
  return { ...state, phaseIndex: 3, lastTransition: 'filter', members: state.members.map((member) => (member.id === targetId ? { ...member, malicious: true, signed: true, signature: committeeMemberSignature('forged-member', state.messageHash) } : member)), aggregateReady: false, aggregateSignature: '', authorized: false };
}

/**
 * filterMalicious 剔除恶意成员签名。
 */
function filterMalicious(state: CommitteeState): CommitteeState {
  let need = Math.max(0, state.threshold - validSignatures(state));
  const members = state.members.map((member) => {
    if (member.malicious) return { ...member, signed: false, active: false, signature: undefined };
    if (member.active && !member.signed && need > 0) {
      need -= 1;
      return { ...member, signed: true, signature: committeeMemberSignature(member.id, state.messageHash) };
    }
    return member;
  });
  return aggregateCommittee({ ...state, phaseIndex: 3, lastTransition: 'filter', members, aggregateReady: false, aggregateSignature: '', authorized: false });
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = committeePhases[index] ?? committeePhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
