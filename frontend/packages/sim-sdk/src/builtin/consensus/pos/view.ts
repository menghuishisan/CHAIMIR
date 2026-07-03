// 本文件把 PoS 内核状态映射为封闭可视化模式。

import type { MatrixCell, ViewSpec } from '../../../types';
import { graphPattern, lanePattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, voteCells, type ViewNode } from '../consensusView';
import { activeStake, attestedStake, labelPosValidator } from './kernel';
import type { PosState } from './model';

/**
 * renderPosView 输出验证者网络、权益见证矩阵和消息泳道。
 */
export function renderPosView(state: PosState): ViewSpec {
  return {
    summary: `Slot ${state.slot},Epoch ${state.epoch},见证权益 ${attestedStake(state)}/${activeStake(state)},最终确定 Epoch ${state.finalizedEpoch}。`,
    patterns: [
      graphPattern('pos-graph', '验证者权益网络', validatorNodes(state), graphEdges(state.messages), 'main'),
      matrixPattern('pos-matrix', '权益见证矩阵', state.validators.map((validator) => validator.label), ['权益', '职责', '见证', '罚没'], posCells(state), 'side'),
      lanePattern('pos-lane', 'PoS 消息时序', state.validators.map((validator) => validator.label), laneMessages(state.messages, (id) => labelPosValidator(state, id)), state.tick, 'bottom'),
    ],
  };
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
      if (column === '权益') return { label: String(validator.stake), status: validator.slashed ? 'fault' : 'yes' };
      if (column === '职责') return { label: validator.proposer ? '提议' : state.committee.includes(validator.id) ? '委员会' : '等待', status: validator.proposer || state.committee.includes(validator.id) ? 'yes' : 'empty' };
      if (column === '见证') return { label: validator.attested ? '已签' : '等待', status: validator.attested ? 'yes' : 'pending' };
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
