// 本文件实现 Tendermint proposal、prevote、precommit、commit 和锁定约束内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { tendermintRoundPhases, type TendermintRoundsState } from './model';
import { traceLinesForTendermintRounds } from './trace';

export function createInitialTendermintRoundsState(_params: SimInitParams, _seed: number): TendermintRoundsState {
  return finalizeTendermintRoundsState({
    tick: 0,
    phase: tendermintRoundPhases[0].label,
    phaseIndex: 0,
    height: 12,
    round: 0,
    validators: [
      { id: 'a', label: 'A', power: 30, online: true },
      { id: 'b', label: 'B', power: 30, online: true },
      { id: 'c', label: 'C', power: 25, online: true },
      { id: 'd', label: 'D', power: 15, online: true },
    ],
    messages: [],
    timeout: false,
    lastTransition: 'proposal',
    explanation: explain(0),
    metrics: {},
    checkpointValues: {},
  });
}

export function reduceTendermintRoundsEvent(state: TendermintRoundsState, event: SimEvent, _context: ReducerContext): TendermintRoundsState {
  if (event.type === 'select') return finalizeTendermintRoundsState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeTendermintRoundsState(forceTimeout(state));
  if (event.type === 'recover') return finalizeTendermintRoundsState(rebroadcastValidValue(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeTendermintRoundsState(advanceTendermint(state, event));
  return state;
}

export function finalizeTendermintRoundsState(state: TendermintRoundsState): TendermintRoundsState {
  const prevotePower = votePower(state, 'prevote');
  const precommitPower = votePower(state, 'precommit');
  return {
    ...state,
    phase: tendermintRoundPhases[state.phaseIndex].label,
    explanation: explain(state.phaseIndex),
    metrics: { result: state.committedValue ? `提交 ${state.committedValue}` : state.timeout ? '等待新轮' : '投票中', risk: state.timeout ? 44 : 9, prevotePower, precommitPower },
    checkpointValues: { committed: Boolean(state.committedValue), lockedCount: state.validators.filter((validator) => validator.lockedValue).length, round: state.round },
    _trace: {
      triggeredLines: traceLinesForTendermintRounds(state.lastTransition),
      variables: { round: state.round, prevotePower, precommitPower, committedValue: state.committedValue ?? '' },
      executionPath: `tendermint/${state.lastTransition}`,
    },
  };
}

export function tendermintRoundsCheckpoint(state: TendermintRoundsState): CheckpointResult {
  return {
    achieved: Boolean(state.committedValue),
    answer: { committedValue: state.committedValue ?? null, lockedCount: state.checkpointValues.lockedCount, round: state.round },
    explanation: state.committedValue ? 'precommit 权重超过 2/3,当前值可以提交。' : '尚未达到 precommit 阈值,必须等待下一轮或更多投票。',
  };
}

export function labelTendermintActor(state: TendermintRoundsState, id: string): string {
  return state.validators.find((validator) => validator.id === id)?.label ?? (id === 'network' ? '网络' : id);
}

function advanceTendermint(state: TendermintRoundsState, event: SimEvent): TendermintRoundsState {
  const tick = event.source === 'tick' ? state.tick + 1 : state.tick;
  if (state.phaseIndex === 0) return propose({ ...state, tick });
  if (state.phaseIndex === 1) return prevote({ ...state, tick });
  if (state.phaseIndex === 2) return precommit({ ...state, tick });
  if (state.phaseIndex === 3) return commit({ ...state, tick });
  if (state.phaseIndex === 4) return nextRound({ ...state, tick });
  return { ...state, tick, phaseIndex: 0, lastTransition: 'proposal' };
}

function propose(state: TendermintRoundsState): TendermintRoundsState {
  const proposer = state.validators[state.round % state.validators.length];
  const value = state.validators.find((validator) => validator.lockedValue)?.lockedValue ?? `block-${state.height}-${state.round}`;
  return {
    ...state,
    phaseIndex: 1,
    lastTransition: 'prevote',
    proposal: { id: `proposal-${state.round}`, proposer: proposer.id, value, round: state.round, valid: true },
    messages: [{ id: `proposal-${state.round}`, from: proposer.id, to: 'network', at: state.tick, label: `proposal ${value}`, status: 'delivered', detail: '提议者广播候选值' }],
  };
}

function prevote(state: TendermintRoundsState): TendermintRoundsState {
  const value = state.proposal?.value;
  return {
    ...state,
    phaseIndex: 2,
    lastTransition: 'precommit',
    validators: state.validators.map((validator) => (validator.online && value ? { ...validator, prevote: value } : validator)),
    messages: [...state.messages, ...state.validators.filter((validator) => validator.online).map((validator) => ({ id: `prevote-${validator.id}-${state.round}`, from: validator.id, to: 'network', at: state.tick + 1, label: `prevote ${value}`, status: 'delivered' as const, detail: '验证者发送 prevote' }))],
  };
}

function precommit(state: TendermintRoundsState): TendermintRoundsState {
  const canLock = votePower(state, 'prevote') >= 67;
  const value = state.proposal?.value;
  return {
    ...state,
    phaseIndex: 3,
    lastTransition: 'commit',
    validators: state.validators.map((validator) => (canLock && validator.prevote === value ? { ...validator, lockedValue: value, precommit: value } : validator)),
    messages: [...state.messages, ...state.validators.filter((validator) => canLock && validator.prevote === value).map((validator) => ({ id: `precommit-${validator.id}-${state.round}`, from: validator.id, to: 'network', at: state.tick + 2, label: `precommit ${value}`, status: 'delivered' as const, detail: '锁定值并发送 precommit' }))],
  };
}

function commit(state: TendermintRoundsState): TendermintRoundsState {
  const committedValue = votePower(state, 'precommit') >= 67 ? state.proposal?.value : undefined;
  return { ...state, phaseIndex: committedValue ? 5 : 4, lastTransition: committedValue ? 'lock' : 'timeout', committedValue, timeout: !committedValue };
}

function nextRound(state: TendermintRoundsState): TendermintRoundsState {
  return { ...state, phaseIndex: 0, round: state.round + 1, timeout: false, lastTransition: 'proposal', validators: state.validators.map((validator) => ({ ...validator, prevote: undefined, precommit: undefined })) };
}

function forceTimeout(state: TendermintRoundsState): TendermintRoundsState {
  return { ...state, phaseIndex: 4, timeout: true, lastTransition: 'timeout', validators: state.validators.map((validator) => (validator.id === 'd' ? { ...validator, online: false } : validator)) };
}

function rebroadcastValidValue(state: TendermintRoundsState): TendermintRoundsState {
  return { ...state, phaseIndex: 0, timeout: false, lastTransition: 'proposal', validators: state.validators.map((validator) => ({ ...validator, online: true })) };
}

function votePower(state: TendermintRoundsState, kind: 'prevote' | 'precommit'): number {
  const value = state.proposal?.value;
  return state.validators.filter((validator) => validator[kind] === value).reduce((sum, validator) => sum + validator.power, 0);
}

function explain(index: number) {
  const phase = tendermintRoundPhases[index] ?? tendermintRoundPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
