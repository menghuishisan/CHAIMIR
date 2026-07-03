// 本文件把零知识证明内核状态映射为封闭可视化模式。

import type { MatrixCell, ViewSpec } from '../../../types';
import { graphPattern, matrixPattern, pipelinePattern } from '../../packageTools';
import { graphEdges, graphNodes, matrixCells, pipelineSteps } from '../cryptoView';
import { zkProofPhases, type ZkState } from './model';

/**
 * renderZkProofView 输出证明者网络、约束矩阵和交互流程。
 */
export function renderZkProofView(state: ZkState): ViewSpec {
  return {
    summary: `挑战 ${state.challenge},承诺 ${state.commitment.slice(0, 8)},验证${state.verifierResult ? '通过' : '未通过'}。`,
    patterns: [
      graphPattern('zk-graph', '证明者与验证者', graphNodes(state.actors), graphEdges(state.messages), 'main'),
      matrixPattern('zk-matrix', '约束检查', ['承诺绑定', '挑战随机', '响应一致', '秘密隐藏'], ['结果'], zkCells(state), 'side'),
      pipelinePattern('zk-pipeline', '零知识交互流程', pipelineSteps([...zkProofPhases], state.phaseIndex, state.cheating && state.phaseIndex >= 4), zkProofPhases[state.phaseIndex].id, 'bottom'),
    ],
  };
}

/**
 * zkCells 展示零知识证明四个约束。
 */
function zkCells(state: ZkState): MatrixCell[][] {
  return matrixCells(['承诺绑定', '挑战随机', '响应一致', '秘密隐藏'], ['结果'], (row) => {
    if (row === '响应一致' && !state.verifierResult && state.phaseIndex >= 4) return { label: '失败', status: 'fault' };
    return { label: row === '秘密隐藏' ? '未泄露' : state.phaseIndex >= 4 ? '通过' : '等待', status: row === '秘密隐藏' || state.phaseIndex >= 4 ? 'yes' : 'pending' };
  });
}
