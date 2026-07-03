// 本文件把跨链多签委员会状态转换为签名矩阵和授权流程。

import type { MatrixCell, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../crossChainView';
import { validSignatures } from './kernel';
import { committeePhases, type CommitteeState } from './model';

/**
 * renderCommitteeView 基于内核状态生成多签委员会可视化。
 */
export function renderCommitteeView(state: CommitteeState): ViewSpec {
  return { summary: `门限 ${state.threshold}/${state.members.length},有效签名 ${validSignatures(state)},授权${state.authorized ? '通过' : '等待'}。`, patterns: [matrixPattern('committee-matrix', '委员会签名', state.members.map((member) => member.label), ['活跃', '签名', '有效'], committeeCells(state), 'main'), pipelinePattern('committee-pipeline', '多签授权流程', pipelineSteps(committeePhases, state.phaseIndex, state.members.some((member) => member.malicious)), committeePhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * committeeCells 展示委员会签名状态。
 */
function committeeCells(state: CommitteeState): MatrixCell[][] {
  return matrixCells(state.members.map((member) => member.label), ['活跃', '签名', '有效'], (row, column) => {
    const member = state.members.find((item) => item.label === row);
    if (!member) return { label: '无', status: 'empty' };
    if (column === '活跃') return { label: member.active ? '是' : '否', status: member.active ? 'yes' : 'fault' };
    if (column === '签名') return { label: member.signed ? '已签' : '等待', status: member.signed ? 'yes' : 'pending' };
    return { label: member.signed && member.active && !member.malicious ? '有效' : member.malicious ? '恶意' : '无', status: member.malicious ? 'fault' : member.signed ? 'yes' : 'empty' };
  });
}
