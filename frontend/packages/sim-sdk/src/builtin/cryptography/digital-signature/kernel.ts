// 本文件实现数字签名的摘要、签名、验签、重放防护和密钥轮换内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import type { CryptoMessage } from '../cryptoView';
import { derivePrivateKey, derivePublicKey, messageDigest, recoverRegisteredPublicKey, signDigest } from '../cryptoPrimitives';
import { digitalSignaturePhases, type SignatureState } from './model';
import { traceLinesForDigitalSignature } from './trace';

const SIGNATURE_DOMAIN = 'chaimir-demo-transfer';

/**
 * createInitialDigitalSignatureState 创建签名者、验证者和重放者场景。
 */
export function createInitialDigitalSignatureState(_params: SimInitParams, _seed: number): SignatureState {
  const signerKey = derivePrivateKey('signer-private');
  const verifierKey = derivePublicKey(signerKey);
  const messageText = '授权转账 10';
  const nonce = 7;
  const digest = messageDigest(SIGNATURE_DOMAIN, messageText, nonce);
  const signature = signDigest(digest, signerKey);
  const keyRegistry = { [verifierKey]: signerKey };
  return finalizeDigitalSignatureState({
    tick: 0,
    phase: digitalSignaturePhases[0].label,
    phaseIndex: 0,
    signerKey,
    verifierKey,
    keyRegistry,
    message: messageText,
    digest,
    signature,
    recoveredKey: verifierKey,
    nonce,
    verified: false,
    replayDetected: false,
    actors: [
      { id: 'sig-signer', label: '签名者', role: 'crypto-actor', status: 'active', value: '私钥持有者' },
      { id: 'sig-verifier', label: '验证者', role: 'crypto-actor', status: 'idle', value: '公钥校验' },
      { id: 'sig-attacker', label: '重放者', role: 'crypto-actor', status: 'idle', value: '监听旧签名' },
    ],
    messages: [],
    lastTransition: 'keypair',
    explanation: explainSignaturePhase(0),
    metrics: {},
    checkpointValues: {},
  });
}

/**
 * reduceDigitalSignatureEvent 是数字签名包唯一事件入口。
 */
export function reduceDigitalSignatureEvent(state: SignatureState, event: SimEvent, _context: ReducerContext): SignatureState {
  if (event.type === 'select') return finalizeDigitalSignatureState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeDigitalSignatureState(replaySignature(state));
  if (event.type === 'recover') return finalizeDigitalSignatureState(rotateKey(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeDigitalSignatureState(advanceSignature(state, event));
  return state;
}

/**
 * advanceSignature 推进签名、发送和验签流程。
 */
export function advanceSignature(state: SignatureState, event: SimEvent): SignatureState {
  const phaseIndex = Math.min(digitalSignaturePhases.length - 1, state.phaseIndex + 1);
  const transition = digitalSignaturePhases[phaseIndex].id;
  const next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: transition };
  if (phaseIndex === 2) return { ...next, messages: next.messages.concat(message('sig-signer', 'sig-verifier', '发送签名', next.tick)) };
  if (phaseIndex === 3) return verifySignature(next);
  if (phaseIndex === 4) return { ...next, replayDetected: false };
  return next;
}

/**
 * signatureValid 输出验签检查点。
 */
export function signatureValid(state: SignatureState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.verified && state.checkpointValues.replayBlocked);
  return { achieved, answer: { verified: state.verified, replayDetected: state.replayDetected, nonce: state.nonce }, explanation: achieved ? '签名有效且 nonce 未被重放。' : '签名尚未通过或存在重放风险。' };
}

/**
 * finalizeDigitalSignatureState 刷新签名流程派生状态。
 */
export function finalizeDigitalSignatureState(state: SignatureState): SignatureState {
  const risk = state.replayDetected ? 80 : state.verified ? 6 : 24;
  return {
    ...state,
    phase: digitalSignaturePhases[state.phaseIndex].label,
    actors: state.actors.map((actor) => ({ ...actor, status: actor.id === 'sig-attacker' && state.replayDetected ? 'danger' : actor.id === 'sig-verifier' && state.verified ? 'success' : actor.status })),
    explanation: explainSignaturePhase(state.phaseIndex),
    metrics: { result: state.verified ? '验签通过' : state.replayDetected ? '重放被拒绝' : '等待验签', risk, nonce: state.nonce },
    checkpointValues: { verified: state.verified, replayBlocked: !state.replayDetected || !state.verified },
    _trace: { triggeredLines: traceLinesForDigitalSignature(state.lastTransition), variables: { digest: state.digest, nonce: state.nonce, verified: state.verified, replayDetected: state.replayDetected }, executionPath: `signature/${state.lastTransition}` },
  };
}

/**
 * verifySignature 使用可信公钥检查签名、消息和 nonce。
 */
function verifySignature(state: SignatureState): SignatureState {
  const recoveredKey = recoverRegisteredPublicKey(state.digest, state.signature, state.keyRegistry);
  return { ...state, recoveredKey, verified: recoveredKey === state.verifierKey && !state.replayDetected };
}

/**
 * replaySignature 模拟攻击者重放旧签名。
 */
function replaySignature(state: SignatureState): SignatureState {
  return { ...state, lastTransition: 'replay', phaseIndex: 4, replayDetected: true, verified: false, messages: state.messages.concat(message('sig-attacker', 'sig-verifier', '重放旧签名', state.tick)) };
}

/**
 * rotateKey 轮换密钥并重新签名。
 */
function rotateKey(state: SignatureState): SignatureState {
  const signerKey = derivePrivateKey(`${state.signerKey}:rotated`);
  const verifierKey = derivePublicKey(signerKey);
  const nonce = state.nonce + 1;
  const digest = messageDigest(SIGNATURE_DOMAIN, state.message, nonce);
  return { ...state, phaseIndex: 5, lastTransition: 'rotate', signerKey, verifierKey, keyRegistry: { [verifierKey]: signerKey }, nonce, digest, signature: signDigest(digest, signerKey), recoveredKey: verifierKey, replayDetected: false, verified: true };
}

/**
 * message 创建签名流程消息。
 */
function message(from: string, to: string, label: string, at: number): CryptoMessage {
  return { id: deterministicId('sig-msg', { from, to, label, at }), from, to, label, at, status: 'delivered' };
}

/**
 * explainSignaturePhase 生成阶段说明。
 */
function explainSignaturePhase(index: number) {
  const phase = digitalSignaturePhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
