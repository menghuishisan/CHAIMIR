// 本文件实现 Ethereum PoS head 选择、checkpoint 证明和最终性推进内核。

import type { ChainBlock, CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { ethPosFinalityPhases, type EthPosAttestation, type EthPosFinalityState } from './model';
import { traceLinesForEthPosFinality } from './trace';

export function createInitialEthPosFinalityState(_params: SimInitParams, _seed: number): EthPosFinalityState {
  const validators = [
    { id: 'v1', label: 'V1', weight: 32, online: true },
    { id: 'v2', label: 'V2', weight: 32, online: true },
    { id: 'v3', label: 'V3', weight: 24, online: true },
    { id: 'v4', label: 'V4', weight: 12, online: true },
  ];
  return finalizeEthPosFinalityState({
    tick: 0,
    phase: ethPosFinalityPhases[0].label,
    phaseIndex: 0,
    slot: 64,
    epoch: 8,
    head: 'b0',
    justified: 'b0',
    finalized: 'genesis',
    validators,
    blocks: [{ id: 'genesis', slot: 0, epoch: 0, parent: '', weight: 100, status: 'finalized' }, { id: 'b0', slot: 63, epoch: 7, parent: 'genesis', weight: 100, status: 'justified' }],
    attestations: [],
    participationHistory: [{ x: 0, quorum: 67, risk: 8, finality: 70 }],
    lastTransition: 'propose',
    explanation: explain(0),
    metrics: {},
    checkpointValues: {},
  });
}

export function reduceEthPosFinalityEvent(state: EthPosFinalityState, event: SimEvent, _context: ReducerContext): EthPosFinalityState {
  if (event.type === 'select') return finalizeEthPosFinalityState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeEthPosFinalityState(delayVote(state));
  if (event.type === 'recover') return finalizeEthPosFinalityState(restoreVote(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeEthPosFinalityState(advanceEthPos(state, event));
  return state;
}

export function finalizeEthPosFinalityState(state: EthPosFinalityState): EthPosFinalityState {
  const total = totalWeight(state);
  const headWeight = blockWeight(state, state.head);
  const finalized = state.blocks.find((block) => block.id === state.finalized);
  return {
    ...state,
    phase: ethPosFinalityPhases[state.phaseIndex].label,
    explanation: explain(state.phaseIndex),
    metrics: { result: finalized ? `最终确定 ${finalized.id}` : '等待最终性', risk: headWeight >= 67 ? 6 : 36, headWeight, totalWeight: total },
    checkpointValues: { head: state.head, justified: state.justified, finalized: state.finalized, headHasQuorum: headWeight >= 67 },
    _trace: {
      triggeredLines: traceLinesForEthPosFinality(state.lastTransition),
      variables: { head: state.head, justified: state.justified, finalized: state.finalized },
      executionPath: `eth-pos/${state.lastTransition}`,
    },
  };
}

export function ethPosFinalityCheckpoint(state: EthPosFinalityState): CheckpointResult {
  return {
    achieved: state.finalized !== 'genesis',
    answer: { head: state.head, justified: state.justified, finalized: state.finalized },
    explanation: state.finalized !== 'genesis' ? '连续 checkpoint 已证明,较早 checkpoint 被 finalized。' : '当前只有 head 或 justified,还没有新的 finalized checkpoint。',
  };
}

export function ethPosChainBlocks(state: EthPosFinalityState): ChainBlock[] {
  return state.blocks.filter((block) => block.parent !== 'b0' || block.id === state.head).map((block) => ({
    id: block.id,
    height: block.slot,
    hash: block.id,
    parentHash: block.parent,
    label: `${block.id} / ${block.weight}`,
    status: block.id === state.finalized ? 'canonical' : block.id === state.head ? 'pending' : block.status === 'orphaned' ? 'orphaned' : block.status === 'finalized' || block.status === 'justified' ? 'canonical' : 'pending',
  }));
}

function advanceEthPos(state: EthPosFinalityState, event: SimEvent): EthPosFinalityState {
  const tick = event.source === 'tick' ? state.tick + 1 : state.tick;
  if (state.phaseIndex === 0) return proposeBlock({ ...state, tick });
  if (state.phaseIndex === 1) return attestHead({ ...state, tick });
  if (state.phaseIndex === 2) return chooseHead({ ...state, tick });
  if (state.phaseIndex === 3) return justifyHead({ ...state, tick });
  if (state.phaseIndex === 4) return finalizeCheckpoint({ ...state, tick });
  return { ...state, tick, phaseIndex: 0, slot: state.slot + 1, lastTransition: 'propose' };
}

function proposeBlock(state: EthPosFinalityState): EthPosFinalityState {
  const id = `b${state.slot + 1}`;
  return { ...state, phaseIndex: 1, lastTransition: 'attest', blocks: [...state.blocks, { id, slot: state.slot + 1, epoch: state.epoch, parent: state.head, weight: 0, status: 'candidate' }] };
}

function attestHead(state: EthPosFinalityState): EthPosFinalityState {
  const target = state.blocks[state.blocks.length - 1]?.id ?? state.head;
  const attestations: EthPosAttestation[] = state.validators.filter((validator) => validator.online).map((validator) => ({ id: `${validator.id}-${target}`, validatorId: validator.id, blockId: target, epoch: state.epoch, delivered: true }));
  return { ...state, phaseIndex: 2, lastTransition: 'ghost', attestations: [...state.attestations, ...attestations], validators: state.validators.map((validator) => (validator.online ? { ...validator, latestVote: target } : validator)) };
}

function chooseHead(state: EthPosFinalityState): EthPosFinalityState {
  const weights = new Map<string, number>();
  for (const validator of state.validators) {
    if (validator.latestVote) weights.set(validator.latestVote, (weights.get(validator.latestVote) ?? 0) + validator.weight);
  }
  const [head] = [...weights.entries()].sort((a, b) => b[1] - a[1])[0] ?? [state.head, 0];
  return { ...state, phaseIndex: 3, head, lastTransition: 'justify', blocks: state.blocks.map((block) => ({ ...block, weight: weights.get(block.id) ?? block.weight, status: block.id === head ? 'head' : block.status })) };
}

function justifyHead(state: EthPosFinalityState): EthPosFinalityState {
  const justified = blockWeight(state, state.head) >= Math.ceil(totalWeight(state) * 2 / 3) ? state.head : state.justified;
  return { ...state, phaseIndex: 4, justified, lastTransition: 'finalize', blocks: state.blocks.map((block) => (block.id === justified ? { ...block, status: 'justified' } : block)) };
}

function finalizeCheckpoint(state: EthPosFinalityState): EthPosFinalityState {
  const justifiedBlock = state.blocks.find((block) => block.id === state.justified);
  const finalized = justifiedBlock?.parent && justifiedBlock.parent !== '' ? justifiedBlock.parent : state.finalized;
  return { ...state, phaseIndex: 5, finalized, lastTransition: 'delay', participationHistory: [...state.participationHistory, { x: state.epoch, quorum: blockWeight(state, state.head), risk: blockWeight(state, state.head) >= 67 ? 5 : 32, finality: finalized !== state.finalized ? 100 : 75 }] };
}

function delayVote(state: EthPosFinalityState): EthPosFinalityState {
  return { ...state, phaseIndex: 5, lastTransition: 'delay', validators: state.validators.map((validator) => (validator.id === 'v4' ? { ...validator, online: false } : validator)) };
}

function restoreVote(state: EthPosFinalityState): EthPosFinalityState {
  return { ...state, phaseIndex: 1, lastTransition: 'attest', validators: state.validators.map((validator) => ({ ...validator, online: true })) };
}

function blockWeight(state: EthPosFinalityState, blockId: string): number {
  return state.validators.filter((validator) => validator.latestVote === blockId).reduce((sum, validator) => sum + validator.weight, 0) || state.blocks.find((block) => block.id === blockId)?.weight || 0;
}

function totalWeight(state: EthPosFinalityState): number {
  return state.validators.reduce((sum, validator) => sum + validator.weight, 0);
}

function explain(index: number) {
  const phase = ethPosFinalityPhases[index] ?? ethPosFinalityPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
