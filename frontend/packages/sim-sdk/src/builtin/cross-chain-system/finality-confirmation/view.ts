// 本文件把跨链最终性状态转换为检查矩阵和最终性流程。

import type { MatrixCell, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../crossChainView';
import { finalityPhases, type FinalityState } from './model';

/**
 * renderFinalityView 基于内核状态生成最终性确认可视化。
 */
export function renderFinalityView(state: FinalityState): ViewSpec {
  return { summary: `确认数 ${state.confirmations}/${state.requiredConfirmations},证明${state.finalityProof ? '已提交' : '等待'},释放${state.released ? '完成' : '未完成'}。`, patterns: [matrixPattern('finality-matrix', '最终性检查', ['确认数', '最终性证明', '重组风险', '释放'], ['结果'], finalityCells(state), 'main'), pipelinePattern('finality-pipeline', '最终性流程', pipelineSteps(finalityPhases, state.phaseIndex, state.reorgDetected), finalityPhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * finalityCells 展示最终性检查项。
 */
function finalityCells(state: FinalityState): MatrixCell[][] {
  const values: Record<string, boolean> = { 确认数: state.confirmations >= state.requiredConfirmations, 最终性证明: state.finalityProof, 重组风险: !state.reorgDetected, 释放: state.released };
  return matrixCells(['确认数', '最终性证明', '重组风险', '释放'], ['结果'], (row) => ({ label: values[row] ? '通过' : row === '重组风险' && state.reorgDetected ? '失败' : '等待', status: values[row] ? 'yes' : row === '重组风险' && state.reorgDetected ? 'fault' : 'pending' }));
}
