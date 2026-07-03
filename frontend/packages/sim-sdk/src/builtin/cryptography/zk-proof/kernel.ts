// 本文件实现零知识证明见证、承诺、挑战、响应、验证和重试恢复内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { integerParam } from '../../initParams';
import type { CryptoMessage } from '../cryptoView';
import { FIELD_PRIME, groupMul, roundDigest, schnorrCommit, schnorrResponse, verifySchnorrRelation } from '../cryptoPrimitives';
import { zkProofPhases, type ZkState } from './model';
import { traceLinesForZkProof } from './trace';

/**
 * createInitialZkProofState 创建证明者、验证者和观察者。
 */
export function createInitialZkProofState(params: SimInitParams, _seed: number): ZkState {
  const secret = integerParam(params, 'secret', 19, 1, FIELD_PRIME - 1);
  const randomizer = integerParam(params, 'randomizer', 11, 1, FIELD_PRIME - 1);
  const publicKey = groupMul(secret);
  const challenge = integerParam(params, 'challenge', 3, 1, FIELD_PRIME - 1);
  const commitment = schnorrCommit(randomizer);
  const responseValue = schnorrResponse(randomizer, challenge, secret);
  return finalizeZkProofState({
    tick: 0,
    phase: zkProofPhases[0].label,
    phaseIndex: 0,
    secret,
    randomizer,
    publicKey,
    commitment,
    challenge,
    response: encodeResponse(responseValue),
    responseValue,
    verifierResult: false,
    cheating: false,
    actors: [
      { id: 'zk-prover', label: '证明者', role: 'crypto-actor', status: 'active', value: '持有见证' },
      { id: 'zk-verifier', label: '验证者', role: 'crypto-actor', status: 'idle', value: '发送挑战' },
      { id: 'zk-observer', label: '观察者', role: 'crypto-actor', status: 'idle', value: '看不到秘密' },
    ],
    messages: [],
    lastTransition: 'witness',
    explanation: explainZkPhase(0),
    metrics: {},
    checkpointValues: {},
  });
}

/**
 * reduceZkProofEvent 是零知识证明包唯一事件入口。
 */
export function reduceZkProofEvent(state: ZkState, event: SimEvent, _context: ReducerContext): ZkState {
  if (event.type === 'select') return finalizeZkProofState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeZkProofState(cheat(state));
  if (event.type === 'recover') return finalizeZkProofState(retry(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeZkProofState(advanceZkProof(state, event));
  return state;
}

/**
 * advanceZkProof 推进零知识交互阶段。
 */
export function advanceZkProof(state: ZkState, event: SimEvent): ZkState {
  const phaseIndex = Math.min(zkProofPhases.length - 1, state.phaseIndex + 1);
  const transition = zkProofPhases[phaseIndex].id;
  const next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: transition };
  if (phaseIndex === 1) return { ...next, messages: next.messages.concat(message('zk-prover', 'zk-verifier', '承诺', next.tick)) };
  if (phaseIndex === 2) return { ...next, messages: next.messages.concat(message('zk-verifier', 'zk-prover', '挑战', next.tick)) };
  if (phaseIndex === 3) return { ...next, messages: next.messages.concat(message('zk-prover', 'zk-verifier', '响应', next.tick)) };
  if (phaseIndex >= 4) return verifyZk(next);
  return next;
}

/**
 * zkProofValid 输出证明检查点。
 */
export function zkProofValid(state: ZkState): CheckpointResult {
  return { achieved: state.verifierResult, answer: { challenge: state.challenge, valid: state.verifierResult }, explanation: state.verifierResult ? '响应满足承诺关系且秘密未暴露。' : '响应与承诺关系不一致。' };
}

/**
 * finalizeZkProofState 刷新零知识证明派生状态。
 */
export function finalizeZkProofState(state: ZkState): ZkState {
  const risk = state.cheating ? 82 : state.verifierResult ? 5 : 22;
  return {
    ...state,
    phase: zkProofPhases[state.phaseIndex].label,
    actors: state.actors.map((actor) => ({ ...actor, status: actor.id === 'zk-prover' && state.cheating ? 'danger' : actor.id === 'zk-verifier' && state.verifierResult ? 'success' : actor.status })),
    explanation: explainZkPhase(state.phaseIndex),
    metrics: { result: state.verifierResult ? '证明通过' : state.cheating ? '伪造被识别' : '等待验证', risk, challenge: state.challenge },
    checkpointValues: { proofValid: state.verifierResult, secretHidden: true },
    _trace: { triggeredLines: traceLinesForZkProof(state.lastTransition), variables: { challenge: state.challenge, response: state.response, verifierResult: state.verifierResult }, executionPath: `zk/${state.lastTransition}` },
  };
}

/**
 * cheat 让证明者在不知道秘密时伪造响应。
 */
function cheat(state: ZkState): ZkState {
  return { ...state, phaseIndex: 4, lastTransition: 'cheat', cheating: true, response: roundDigest('forged-response', String(state.responseValue), 12), responseValue: (state.responseValue + 9) % FIELD_PRIME };
}

/**
 * retry 用新挑战重新生成有效响应。
 */
function retry(state: ZkState): ZkState {
  const challenge = state.challenge + 1;
  const randomizer = state.randomizer + 5;
  const responseValue = schnorrResponse(randomizer, challenge, state.secret);
  return verifyZk({ ...state, phaseIndex: 5, lastTransition: 'repeat', randomizer, commitment: schnorrCommit(randomizer), challenge, cheating: false, response: encodeResponse(responseValue), responseValue });
}

/**
 * verifyZk 检查承诺、挑战和响应是否一致。
 */
function verifyZk(state: ZkState): ZkState {
  return { ...state, verifierResult: !state.cheating && verifySchnorrRelation(state.commitment, state.challenge, state.publicKey, state.responseValue) };
}

/**
 * encodeResponse 将响应值转成界面展示的短摘要。
 */
function encodeResponse(value: number): string {
  return roundDigest('zk-response', String(value), 12);
}

/**
 * message 创建交互消息。
 */
function message(from: string, to: string, label: string, at: number): CryptoMessage {
  return { id: deterministicId('zk-msg', { from, to, label, at }), from, to, label, at, status: 'delivered' };
}

/**
 * explainZkPhase 生成阶段说明。
 */
function explainZkPhase(index: number) {
  const phase = zkProofPhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
