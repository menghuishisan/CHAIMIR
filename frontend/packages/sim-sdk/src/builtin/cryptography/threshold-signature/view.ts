// 本文件把门限签名内核状态映射为封闭可视化模式。

import type { MatrixCell, TeachingFrame } from '../../../types';
import { teachingFrame, graphPattern, matrixPattern, pipelinePattern, selectedOrFrameFocus } from '../../packageTools';
import { graphEdges, graphNodes, matrixCells, pipelineSteps } from '../cryptoView';
import { validShares } from './kernel';
import { thresholdSignaturePhases, type ThresholdState } from './model';

/**
 * renderThresholdSignatureView 输出份额签名网络、份额矩阵和门限流程。
 */
export function renderThresholdSignatureView(state: ThresholdState): TeachingFrame {
  const valid = validShares(state);
    const summary = `门限 ${state.threshold}/${state.holders.length},有效部分签名 ${valid},还差 ${Math.max(0, state.threshold - valid)},聚合${state.aggregateValid ? '通过' : '等待'}。`;
  const patterns = [
      graphPattern('threshold-graph', `份额签名收集网络,有效 ${valid}/${state.threshold}`, graphNodes(state.holders), graphEdges(state.messages)),
      matrixPattern('threshold-matrix', '门限份额与部分签名矩阵', state.holders.map((holder) => holder.label), ['份额', '部分签名', '有效性'], shareCells(state)),
      pipelinePattern('threshold-pipeline', '分发份额 -> 收集部分签名 -> 聚合验证流程', pipelineSteps([...thresholdSignaturePhases], state.phaseIndex, valid < state.threshold && state.phaseIndex >= 4), thresholdSignaturePhases[state.phaseIndex].id),
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
      primary: selectedOrFrameFocus(state.selectedElementId, ['threshold-pipeline']),
      secondary: ['threshold-graph', 'threshold-matrix'],
    },
    layout: {
      primary: 'threshold-pipeline',
      evidence: ['threshold-graph', 'threshold-matrix'],
    },
    patterns,
  });
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
