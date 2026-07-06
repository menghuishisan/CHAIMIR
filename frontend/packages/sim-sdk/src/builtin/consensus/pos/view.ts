// 本文件把 PoS 内核状态映射为封闭可视化模式。

import type { MatrixCell, TeachingFrame } from '../../../types';
import { teachingFrame, graphPattern, lanePattern, matrixPattern, selectedOrFrameFocus } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, voteCells, type ViewNode } from '../consensusView';
import { activeStake, attestedStake, labelPosValidator } from './kernel';
import type { PosState } from './model';

/**
 * renderPosView 输出验证者网络、权益见证矩阵和消息泳道。
 */
export function renderPosView(state: PosState): TeachingFrame {
  const totalStake = activeStake(state);
  const signedStake = attestedStake(state);
  const slashed = state.validators.filter((validator) => validator.slashed).length;
    const summary = `Slot ${state.slot},Epoch ${state.epoch},见证权益 ${signedStake}/${totalStake},委员会 ${state.committee.length} 人,罚没 ${slashed} 人,最终确定 Epoch ${state.finalizedEpoch}。`;
  const patterns = [
      graphPattern('pos-graph', `验证者权益网络,已见证 ${signedStake}/${totalStake}`, validatorNodes(state), graphEdges(state.messages)),
      matrixPattern('pos-matrix', '权益加权见证矩阵', state.validators.map((validator) => validator.label), ['权益权重', '职责', '见证签名', '罚没证据'], posCells(state)),
      lanePattern('pos-lane', 'PoS 提议与见证消息时序', state.validators.map((validator) => validator.label), laneMessages(state.messages, (id) => labelPosValidator(state, id)), state.tick),
    ];
  return teachingFrame({
    summary,
    phase: {
      id: state.phase,
      title: state.explanation.title,
      intent: 'observe',
      what: state.explanation.effect,
      why: state.explanation.reason,
      watch: summary,
    },
    focus: {
      primary: selectedOrFrameFocus(state.selectedElementId, ['pos-matrix']),
      secondary: ['pos-graph', 'pos-lane'],
    },
    layout: {
      primary: 'pos-matrix',
      evidence: ['pos-graph'],
      timeline: 'pos-lane',
    },
    patterns,
  });
}

/**
 * validatorNodes 将权益、提议者和罚没状态映射为图节点。
 */
function validatorNodes(state: PosState): ViewNode[] {
  return graphNodes(state.validators.map((validator) => ({ id: validator.id, label: validator.label, role: 'validator', status: validator.slashed ? 'danger' : validator.proposer ? 'active' : validator.attested ? 'success' : 'idle', value: `权益 ${validator.stake}` })));
}

/**
 * posCells 展示权益、提议、见证和罚没状态。
 */
function posCells(state: PosState): MatrixCell[][] {
  return voteCells(
    state.validators.map((validator) => validator.label),
    ['权益', '职责', '见证', '罚没'],
    (row, column) => {
      const validator = state.validators.find((item) => item.label === row);
      if (!validator) return { label: '无', status: 'empty' };
      if (column === '权益权重') return { label: `${validator.stake}/${activeStake(state)}`, status: validator.slashed ? 'fault' : 'yes' };
      if (column === '职责') return { label: validator.proposer ? '提议' : state.committee.includes(validator.id) ? '委员会' : '等待', status: validator.proposer || state.committee.includes(validator.id) ? 'yes' : 'empty' };
      if (column === '见证签名') return { label: validator.attested ? '已签' : '等待', status: validator.attested ? 'yes' : 'pending' };
      const slashing = state.slashings.find((item) => item.validatorId === validator.id);
      return { label: validator.slashed ? slashingReasonLabel(slashing?.reason) : '正常', status: validator.slashed ? 'fault' : 'yes' };
    }
  );
}

/**
 * slashingReasonLabel 把罚没证据类型转换为面向学习者的简短标签。
 */
function slashingReasonLabel(reason?: string): string {
  if (reason === 'surround-vote') return '包围投票';
  if (reason === 'double-vote') return '双签';
  return '已罚没';
}
