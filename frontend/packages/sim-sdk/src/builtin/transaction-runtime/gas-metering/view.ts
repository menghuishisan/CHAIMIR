// 本文件把 Gas 计量状态转换为指令矩阵和执行流程。

import type { MatrixCell, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../runtimeView';
import { gasPhases, type GasState } from './model';

/**
 * renderGasView 基于内核状态生成 Gas 计量可视化。
 */
export function renderGasView(state: GasState): ViewSpec {
  const remaining = Math.max(0, state.gasLimit - state.gasUsed);
  const executed = state.steps.filter((step) => step.executed).length;
  return { summary: `Gas 上限 ${state.gasLimit},已用 ${state.gasUsed},剩余 ${remaining},退款 ${state.refund},已执行 ${executed}/${state.steps.length},状态 ${state.outOfGas ? '失败' : '运行中'}。`, patterns: [matrixPattern('gas-matrix', 'EVM 指令逐步扣 Gas 矩阵', state.steps.map((step) => step.op), ['成本', '执行', '剩余预算影响'], gasCells(state), 'main'), pipelinePattern('gas-pipeline', '设置上限 -> 逐指令扣费 -> 退款/结算流程', pipelineSteps(gasPhases, state.phaseIndex, state.outOfGas), gasPhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * gasCells 展示指令 gas 状态。
 */
function gasCells(state: GasState): MatrixCell[][] {
  return matrixCells(state.steps.map((step) => step.op), ['成本', '执行', '剩余预算影响'], (row, column) => {
    const step = state.steps.find((item) => item.op === row);
    if (!step) return { label: '无', status: 'empty' };
    if (column === '成本') return { label: String(step.cost), status: 'yes' };
    if (column === '执行') return { label: step.executed ? '已执行' : '等待', status: step.executed ? 'yes' : 'pending' };
    return { label: step.failed ? 'out of gas' : step.executed ? '已扣费' : '未扣费', status: step.failed ? 'fault' : step.executed ? 'yes' : 'pending' };
  });
}
