// 本文件实现 PoS 随机选主、权益见证、最终性和罚没内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { aggregateConsensusSignatures, canonicalConsensusDigest, weightedTwoThirdsThreshold } from '../consensusPrimitives';
import { processViewMessage, refreshViewMessages, type ViewMessage } from '../consensusView';
import { posPhases, type PosAttestation, type PosSlashing, type PosState, type PosValidator } from './model';
import { traceLinesForPos } from './trace';

/**
 * createInitialPosState 创建 PoS 验证者集合和初始 epoch。
 */
export function createInitialPosState(_params: SimInitParams, _seed: number): PosState {
  const validators: PosValidator[] = [
    { id: 'pos-v1', label: '验证者 A', stake: 32, proposer: false, attested: false, slashed: false, online: true },
    { id: 'pos-v2', label: '验证者 B', stake: 28, proposer: false, attested: false, slashed: false, online: true },
    { id: 'pos-v3', label: '验证者 C', stake: 24, proposer: false, attested: false, slashed: false, online: true },
    { id: 'pos-v4', label: '验证者 D', stake: 16, proposer: false, attested: false, slashed: false, online: true },
  ];
  return finalizePosState({
    tick: 0,
    phase: posPhases[0].label,
    phaseIndex: 0,
    slot: 64,
    epoch: 8,
    randomness: canonicalConsensusDigest('pos-randao', { epoch: 8, slot: 64 }, 16),
    blockRoot: canonicalConsensusDigest('pos-block', { epoch: 8, slot: 64 }, 16),
    committee: [],
    validators,
    attestations: [],
    slashings: [],
    messages: [],
    justifiedEpoch: 7,
    finalizedEpoch: 6,
    samples: [{ x: 0, quorum: 67, risk: 10, finality: 25 }],
    lastTransition: posPhases[0].id,
    explanation: explainPosPhase(0),
    metrics: {},
    checkpointValues: {},
  });
}

/**
 * reducePosEvent 是 PoS 仿真包唯一事件入口。
 */
export function reducePosEvent(state: PosState, event: SimEvent, _context: ReducerContext): PosState {
  if (event.type === 'select') return finalizePosState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizePosState(injectEquivocation(state));
  if (event.type === 'recover') return finalizePosState(slashEquivocators(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizePosState(advancePos(state));
  return state;
}

/**
 * advancePos 按 PoS 最终性流程推进一个过程单元。
 */
export function advancePos(state: PosState): PosState {
  const phaseIndex = Math.min(posPhases.length - 1, state.phaseIndex + (state.lastTransition === posPhases[state.phaseIndex].id ? 1 : 0));
  const next = { ...state, phaseIndex, tick: state.tick + 1 };
  if (phaseIndex === 1) return chooseProposer(next);
  if (phaseIndex === 2) return proposeBlock(next);
  if (phaseIndex === 3) return attestBlock(next);
  if (phaseIndex === 4) return justifyCheckpoint(next);
  if (phaseIndex === 5) return finalizeCheckpoint(next);
  if (phaseIndex === 6) return slashEquivocators(next);
  return next;
}

/**
 * posTwoThirdsFinality 检查权益见证和最终性条件。
 */
export function posTwoThirdsFinality(state: PosState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.twoThirds && state.checkpointValues.finalized);
  return { achieved, answer: { attestedStake: attestedStake(state), threshold: twoThirds(activeStake(state)), finalizedEpoch: state.finalizedEpoch }, explanation: achieved ? '三分之二以上权益完成见证并最终确定检查点。' : '仍需足够权益见证或连续证明。' };
}

/**
 * posSlashingHandled 检查冲突见证是否已罚没。
 */
export function posSlashingHandled(state: PosState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.slashingHandled);
  return { achieved, answer: { slashed: state.validators.filter((validator) => validator.slashed).map((validator) => validator.label) }, explanation: achieved ? '冲突见证已被罚没或当前没有冲突。' : '存在未处理的双签冲突。' };
}

/**
 * finalizePosState 刷新 PoS 指标、消息过程、检查点和追踪。
 */
export function finalizePosState(state: PosState): PosState {
  const risk = state.conflictingRoot ? 78 : state.validators.some((validator) => validator.slashed) ? 22 : 10;
  const finality = Math.min(100, Math.max(0, (state.finalizedEpoch - 5) * 25));
  const samples = state.samples.concat({ x: state.tick + state.phaseIndex, quorum: Math.round((attestedStake(state) / Math.max(1, activeStake(state))) * 100), risk, finality }).slice(-24);
  return {
    ...state,
    phase: posPhases[state.phaseIndex].label,
    explanation: explainPosPhase(state.phaseIndex),
    messages: refreshViewMessages(state.messages, state.tick, (message) => message.detail ?? `${message.label} 在验证者集合中传播。`),
    samples,
    metrics: { result: state.finalizedEpoch >= state.epoch - 1 ? '检查点已最终确定' : '等待权益见证', risk, attestedStake: attestedStake(state), activeStake: activeStake(state), finalizedEpoch: state.finalizedEpoch },
    checkpointValues: { twoThirds: attestedStake(state) >= twoThirds(activeStake(state)), finalized: state.finalizedEpoch >= state.epoch - 1, slashingHandled: !state.conflictingRoot && state.slashings.length === 0 },
    _trace: { triggeredLines: traceLinesForPos(state.lastTransition), variables: { slot: state.slot, epoch: state.epoch, blockRoot: state.blockRoot, committeeSize: state.committee.length, aggregateSignature: state.aggregateSignature ?? '' }, executionPath: `pos/${state.lastTransition}` },
  };
}

/**
 * chooseProposer 按权益权重确定提议者。
 */
function chooseProposer(state: PosState): PosState {
  const total = activeStake(state);
  const cursor = Number.parseInt(state.randomness.slice(0, 4), 16) % total;
  let sum = 0;
  const proposer = state.validators.find((validator) => {
    sum += validator.stake;
    return cursor < sum;
  }) ?? state.validators[0];
  const committee = selectCommittee(state, proposer.id);
  return { ...state, lastTransition: 'proposer', committee, validators: state.validators.map((validator) => ({ ...validator, proposer: validator.id === proposer.id, attested: false })) };
}

/**
 * proposeBlock 由提议者广播区块根。
 */
function proposeBlock(state: PosState): PosState {
  const proposer = state.validators.find((validator) => validator.proposer) ?? state.validators[0];
  return { ...state, lastTransition: 'propose', blockRoot: canonicalConsensusDigest('pos-proposed-block', { proposerId: proposer.id, randomness: state.randomness, slot: state.slot }, 16), messages: state.messages.concat(broadcast(state, proposer.id, '提议区块')) };
}

/**
 * attestBlock 让在线且未罚没验证者对区块根签名见证。
 */
function attestBlock(state: PosState): PosState {
  const committee = state.committee.length > 0 ? state.committee : state.validators.filter((validator) => validator.online && !validator.slashed).map((validator) => validator.id);
  const attestations: PosAttestation[] = state.validators
    .filter((validator) => committee.includes(validator.id) && validator.online && !validator.slashed)
    .map((validator) => ({ validatorId: validator.id, blockRoot: state.blockRoot, sourceEpoch: state.justifiedEpoch, targetEpoch: state.epoch, signature: signAttestation(validator.id, state.blockRoot, state.justifiedEpoch, state.epoch), valid: true }));
  return { ...state, lastTransition: 'attest', attestations, aggregateSignature: aggregateSignature(attestations), validators: state.validators.map((validator) => ({ ...validator, attested: attestations.some((attestation) => attestation.validatorId === validator.id) })) };
}

/**
 * justifyCheckpoint 在见证权益达到三分之二后证明当前 epoch。
 */
function justifyCheckpoint(state: PosState): PosState {
  const justified = attestedStake(state) >= twoThirds(activeStake(state)) && state.attestations.every((attestation) => attestation.sourceEpoch <= attestation.targetEpoch);
  return justified ? { ...state, lastTransition: 'justify', justifiedEpoch: state.epoch } : { ...state, lastTransition: 'justify' };
}

/**
 * finalizeCheckpoint 在连续证明后最终确定前一 epoch。
 */
function finalizeCheckpoint(state: PosState): PosState {
  return state.justifiedEpoch === state.epoch ? { ...state, lastTransition: 'finalize', finalizedEpoch: state.epoch - 1 } : { ...state, lastTransition: 'finalize' };
}

/**
 * injectEquivocation 注入提议者双签冲突区块根。
 */
function injectEquivocation(state: PosState): PosState {
  const proposer = state.validators.find((validator) => validator.proposer) ?? state.validators[0];
  const conflictingRoot = canonicalConsensusDigest('pos-conflicting-block', { blockRoot: state.blockRoot, proposerId: proposer.id }, 16);
  const conflict = { validatorId: proposer.id, blockRoot: conflictingRoot, sourceEpoch: state.justifiedEpoch, targetEpoch: state.epoch, signature: signAttestation(proposer.id, conflictingRoot, state.justifiedEpoch, state.epoch), valid: false };
  return { ...state, tick: state.tick + 1, lastTransition: 'slash', conflictingRoot, attestations: state.attestations.concat(conflict), messages: state.messages.concat(broadcast(state, proposer.id, '冲突提议')) };
}

/**
 * slashEquivocators 识别同一验证者的冲突见证并执行罚没。
 */
function slashEquivocators(state: PosState): PosState {
  const slashings = detectSlashings(state.attestations);
  const offenders = new Set(slashings.map((slashing) => slashing.validatorId));
  return { ...state, tick: state.tick + 1, lastTransition: 'slash', slashings: [], validators: state.validators.map((validator) => (offenders.has(validator.id) ? { ...validator, slashed: true, online: false, proposer: false, attested: false } : validator)), conflictingRoot: offenders.size > 0 ? undefined : state.conflictingRoot };
}

/**
 * broadcast 创建验证者广播消息。
 */
function broadcast(state: PosState, from: string, label: string): ViewMessage[] {
  return state.validators
    .filter((validator) => validator.id !== from)
    .map((validator) =>
      processViewMessage(state.tick, { id: deterministicId('pos-msg', { from, to: validator.id, label, tick: state.tick }), from, to: validator.id, at: state.tick, label, status: validator.online ? 'delivered' : 'dropped' }, `${label} 在验证者集合中传播。`)
    );
}

/**
 * activeStake 统计未罚没且在线的权益。
 */
export function activeStake(state: PosState): number {
  return state.validators.filter((validator) => validator.online && !validator.slashed).reduce((sum, validator) => sum + validator.stake, 0);
}

/**
 * attestedStake 统计当前有效见证权益。
 */
export function attestedStake(state: PosState): number {
  const attesters = new Set(state.attestations.filter((attestation) => attestation.valid && attestation.blockRoot === state.blockRoot).map((attestation) => attestation.validatorId));
  return state.validators.filter((validator) => attesters.has(validator.id) && !validator.slashed).reduce((sum, validator) => sum + validator.stake, 0);
}

/**
 * twoThirds 计算三分之二权益阈值。
 */
export function twoThirds(total: number): number {
  return weightedTwoThirdsThreshold(total);
}

/**
 * labelPosValidator 把验证者 ID 转成展示标签。
 */
export function labelPosValidator(state: PosState, id: string): string {
  return state.validators.find((validator) => validator.id === id)?.label ?? id;
}

/**
 * selectCommittee 基于随机种子确定当前 slot 的见证委员会。
 */
function selectCommittee(state: PosState, proposerId: string): string[] {
  return state.validators
    .filter((validator) => validator.online && !validator.slashed && validator.id !== proposerId)
    .sort((left, right) => canonicalConsensusDigest('pos-committee', { randomness: state.randomness, validatorId: left.id }, 8).localeCompare(canonicalConsensusDigest('pos-committee', { randomness: state.randomness, validatorId: right.id }, 8)))
    .map((validator) => validator.id);
}

/**
 * signAttestation 生成可复算的教学签名摘要。
 */
function signAttestation(validatorId: string, blockRoot: string, sourceEpoch: number, targetEpoch: number): string {
  return canonicalConsensusDigest('pos-attestation-signature', { blockRoot, sourceEpoch, targetEpoch, validatorId }, 16);
}

/**
 * aggregateSignature 聚合委员会签名,供代码追踪变量展示。
 */
function aggregateSignature(attestations: PosAttestation[]): string {
  return aggregateConsensusSignatures('pos-aggregate-attestation', attestations.map((attestation) => attestation.signature));
}

/**
 * detectSlashings 同时检测同目标双签与 Casper FFG surround vote。
 */
function detectSlashings(attestations: PosAttestation[]): PosSlashing[] {
  const byValidator = new Map<string, PosAttestation[]>();
  for (const attestation of attestations) {
    byValidator.set(attestation.validatorId, (byValidator.get(attestation.validatorId) ?? []).concat(attestation));
  }
  const slashings: PosSlashing[] = [];
  for (const [validatorId, votes] of byValidator.entries()) {
    for (let leftIndex = 0; leftIndex < votes.length; leftIndex += 1) {
      for (let rightIndex = leftIndex + 1; rightIndex < votes.length; rightIndex += 1) {
        const left = votes[leftIndex];
        const right = votes[rightIndex];
        const doubleVote = left.targetEpoch === right.targetEpoch && left.blockRoot !== right.blockRoot;
        const surroundVote = left.sourceEpoch < right.sourceEpoch && right.targetEpoch < left.targetEpoch;
        if (doubleVote || surroundVote) {
          slashings.push({ validatorId, reason: doubleVote ? 'double-vote' : 'surround-vote', evidenceRoots: [left.blockRoot, right.blockRoot] });
        }
      }
    }
  }
  return slashings;
}

/**
 * explainPosPhase 生成当前阶段说明。
 */
function explainPosPhase(index: number) {
  const phase = posPhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
