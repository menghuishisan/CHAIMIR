// 本文件把 Gas 计量状态转换为指令矩阵和执行流程。

import type { MatrixCell, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../runtimeView';
import { gasPhases, type GasState } from './model';

/**
 * renderGasView 基于内核状态生成 Gas 计量可视化。
 */
export function renderGasView(state: GasState): ViewSpec {
  return { summary: `gasLimit ${state.gasLimit},gasUsed ${state.gasUsed},退款 ${state.refund},状态 ${state.outOfGas ? '失败' : '运行中'}。`, patterns: [matrixPattern('gas-matrix', '指令 gas 表', state.steps.map((step) => step.op), ['成本', '执行', '状态'], gasCells(state), 'main'), pipelinePattern('gas-pipeline', 'Gas 执行流程', pipelineSteps(gasPhases, state.phaseIndex, state.outOfGas), gasPhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * gasCells 展示指令 gas 状态。
 */
function gasCells(state: GasState): MatrixCell[][] {
  return matrixCells(state.steps.map((step) => step.op), ['成本', '执行', '状态'], (row, column) => {
    const step = state.steps.find((item) => item.op === row);
    if (!step) return { label: '无', status: 'empty' };
    if (column === '成本') return { label: String(step.cost), status: 'yes' };
    if (column === '执行') return { label: step.executed ? '已执行' : '等待', status: step.executed ? 'yes' : 'pending' };
    return { label: step.failed ? '失败' : '正常', status: step.failed ? 'fault' : 'yes' };
  });
}
