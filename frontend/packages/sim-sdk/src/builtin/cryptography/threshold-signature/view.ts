// 本文件把门限签名内核状态映射为封闭可视化模式。

import type { MatrixCell, ViewSpec } from '../../../types';
import { graphPattern, matrixPattern, pipelinePattern } from '../../packageTools';
import { graphEdges, graphNodes, matrixCells, pipelineSteps } from '../cryptoView';
import { validShares } from './kernel';
import { thresholdSignaturePhases, type ThresholdState } from './model';

/**
 * renderThresholdSignatureView 输出份额签名网络、份额矩阵和门限流程。
 */
export function renderThresholdSignatureView(state: ThresholdState): ViewSpec {
  return {
    summary: `门限 ${state.threshold}/${state.holders.length},有效部分签名 ${validShares(state)},聚合${state.aggregateValid ? '通过' : '等待'}。`,
    patterns: [
      graphPattern('threshold-graph', '份额签名网络', graphNodes(state.holders), graphEdges(state.messages), 'main'),
      matrixPattern('threshold-matrix', '份额状态', state.holders.map((holder) => holder.label), ['份额', '部分签名', '有效性'], shareCells(state), 'side'),
      pipelinePattern('threshold-pipeline', '门限签名流程', pipelineSteps([...thresholdSignaturePhases], state.phaseIndex, validShares(state) < state.threshold && state.phaseIndex >= 4), thresholdSignaturePhases[state.phaseIndex].id, 'bottom'),
    ],
  };
}

/**
 * shareCells 展示每个份额是否签名且有效。
 */
function shareCells(state: ThresholdState): MatrixCell[][] {
  return matrixCells(state.holders.map((holder) => holder.label), ['份额', '部分签名', '有效性'], (row, column) => {
    const holder = state.holders.find((item) => item.label === row);
    if (!holder) return { label: '无', status: 'empty' };
    if (column === '份额') return { label: holder.share.slice(0, 4), status: holder.faulty ? 'fault' : 'yes' };
    if (column === '部分签名') return { label: holder.partialSignature ? holder.partialSignature.slice(0, 6) : '等待', status: holder.signed ? 'yes' : 'pending' };
    return { label: holder.faulty ? '故障' : '有效', status: holder.faulty ? 'fault' : 'yes' };
  });
}
