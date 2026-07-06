// 本文件把 Tendermint 轮次状态映射为时序泳道、投票矩阵和消息图。

import type { GraphNode, MatrixCell, TeachingFrame, VisualElementMeta } from '../../../types';
import { graphPattern, lanePattern, matrixPattern, selectedOrFrameFocus, teachingFrame } from '../../packageTools';
import { graphEdges, laneMessages, voteCells } from '../consensusView';
import { labelTendermintActor } from './kernel';
import { tendermintRoundPhases, type TendermintRoundsState } from './model';

export function renderTendermintRoundsView(state: TendermintRoundsState): TeachingFrame {
  const summary = `高度 ${state.height},Round ${state.round},提议 ${state.proposal?.value ?? '等待'},已提交 ${state.committedValue ?? '未提交'},超时 ${state.timeout ? '是' : '否'}。`;
  const primary = state.phaseIndex <= 3 ? 'tendermint-lane' : 'tendermint-matrix';
  return teachingFrame({
    summary,
    phase: {
      id: tendermintRoundPhases[state.phaseIndex].id,
      title: state.explanation.title,
      intent: state.timeout ? 'recover' : state.phaseIndex >= 2 ? 'verify' : 'observe',
      what: state.explanation.effect,
      why: state.explanation.reason,
      watch: summary,
    },
    focus: {
      primary: selectedOrFrameFocus(state.selectedElementId, [state.proposal?.id ?? 'tendermint-lane']),
      secondary: state.validators.filter((validator) => validator.lockedValue).map((validator) => validator.id),
      muted: state.validators.filter((validator) => !validator.online).map((validator) => validator.id),
    },
    layout: { primary, evidence: ['tendermint-matrix'], timeline: 'tendermint-lane' },
    patterns: [
      lanePattern('tendermint-lane', 'Proposal -> Prevote -> Precommit -> Commit 时序', state.validators.map((validator) => validator.label).concat('网络'), laneMessages(state.messages, (id) => labelTendermintActor(state, id)), state.tick),
      matrixPattern('tendermint-matrix', '验证者投票权重和锁定状态', state.validators.map((validator) => validator.label), ['权重', 'Prevote', 'Precommit', 'Lock'], voteMatrix(state)),
      graphPattern('tendermint-graph', 'Tendermint 广播网络', graphNodes(state), graphEdges(state.messages)),
    ],
  });
}

function voteMatrix(state: TendermintRoundsState): MatrixCell[][] {
  return voteCells(state.validators.map((validator) => validator.label), ['权重', 'Prevote', 'Precommit', 'Lock'], (row, column) => {
    const validator = state.validators.find((item) => item.label === row);
    if (!validator) return { label: '无', status: 'empty' };
    const cellMeta = meta(validator.id, validator.label, validator.lockedValue ? 'focus' : validator.online ? 'context' : 'ghost', state.tick);
    if (column === '权重') return { label: String(validator.power), status: 'yes', meta: cellMeta };
    if (column === 'Prevote') return { label: validator.prevote ?? '等待', status: validator.prevote ? 'yes' : 'pending', meta: cellMeta };
    if (column === 'Precommit') return { label: validator.precommit ?? '等待', status: validator.precommit ? 'yes' : 'pending', meta: cellMeta };
    return { label: validator.lockedValue ?? '未锁定', status: validator.lockedValue ? 'yes' : 'pending', meta: cellMeta };
  });
}

function graphNodes(state: TendermintRoundsState): GraphNode[] {
  const validatorNodes: GraphNode[] = state.validators.map((validator) => ({ id: validator.id, label: validator.label, role: 'validator', status: validator.lockedValue ? 'success' : validator.online ? 'active' : 'warning', value: `power ${validator.power}`, meta: meta(validator.id, validator.label, validator.lockedValue ? 'focus' : 'context', state.tick) }));
  return validatorNodes.concat({ id: 'network', label: '网络', role: 'broadcast', status: state.timeout ? 'warning' : 'active', meta: meta('network', '网络', 'context', state.tick) });
}

function meta(id: string, label: string, emphasis: VisualElementMeta['emphasis'], tick: number): VisualElementMeta {
  return { id, label, lifecycle: { state: emphasis === 'ghost' ? 'archived' : emphasis === 'focus' ? 'active' : 'settled', fromTick: Math.max(0, tick - 1) }, emphasis, explanation: label };
}
