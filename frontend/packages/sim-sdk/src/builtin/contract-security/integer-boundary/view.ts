// 本文件把整数边界状态转换为边界用例矩阵和安全流程。

import type { MatrixCell, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../securityView';
import { integerPhases, type IntegerBoundaryState } from './model';

/**
 * renderIntegerView 基于内核状态生成整数边界可视化。
 */
export function renderIntegerView(state: IntegerBoundaryState): ViewSpec {
  return { summary: `最大值 ${state.maxValue},checked ${state.checkedMath ? '已启用' : '未启用'},失败用例 ${state.cases.filter((item) => item.failed).length} 个。`, patterns: [matrixPattern('integer-matrix', '边界用例', state.cases.map((item) => item.label), ['输入', '结果', 'checked', '状态'], integerCells(state), 'main'), pipelinePattern('integer-pipeline', '整数安全流程', pipelineSteps(integerPhases, state.phaseIndex, state.cases.some((item) => item.failed)), integerPhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * integerCells 展示每个边界用例。
 */
function integerCells(state: IntegerBoundaryState): MatrixCell[][] {
  return matrixCells(state.cases.map((item) => item.label), ['输入', '结果', 'checked', '状态'], (row, column) => {
    const item = state.cases.find((entry) => entry.label === row);
    if (!item) return { label: '无', status: 'empty' };
    if (column === '输入') return { label: String(item.input), status: item.input > state.maxValue ? 'fault' : 'yes' };
    if (column === '结果') return { label: String(item.result), status: item.failed ? 'fault' : 'yes' };
    if (column === 'checked') return { label: state.checkedMath ? '是' : '否', status: state.checkedMath ? 'yes' : 'pending' };
    return { label: item.failed ? '已拒绝' : '通过', status: item.failed && state.checkedMath ? 'yes' : item.failed ? 'fault' : 'yes' };
  });
}
