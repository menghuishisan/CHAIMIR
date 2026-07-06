// 本文件把区块验证状态转换为验证矩阵和验证流程。

import type { MatrixCell, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../runtimeView';
import { blockValidationPhases, type BlockValidationState } from './model';

/**
 * renderBlockValidationView 基于内核状态生成区块验证可视化。
 */
export function renderBlockValidationView(state: BlockValidationState): ViewSpec {
  const failedItems = state.items.filter((item) => !item.valid);
  return { summary: `区块 ${state.blockHash.slice(0, 8)},验证项 ${state.items.length},失败 ${failedItems.length},校验${state.accepted ? '通过' : '未通过'}。`, patterns: [matrixPattern('block-validation-matrix', '区块头 / 交易根 / 收据根 / 状态根校验矩阵', state.items.map((item) => item.label), ['期望根', '实际根', '状态'], validationCells(state), 'main'), pipelinePattern('block-validation-pipeline', '本地重算各根并拒绝无效区块流程', pipelineSteps(blockValidationPhases, state.phaseIndex, state.items.some((item) => !item.valid)), blockValidationPhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * validationCells 展示验证项状态。
 */
function validationCells(state: BlockValidationState): MatrixCell[][] {
  return matrixCells(state.items.map((item) => item.label), ['期望根', '实际根', '状态'], (row, column) => {
    const item = state.items.find((entry) => entry.label === row);
    if (!item) return { label: '无', status: 'empty' };
    if (column === '期望根') return { label: item.expected.slice(0, 6), status: 'yes' };
    if (column === '实际根') return { label: item.actual.slice(0, 6), status: item.valid ? 'yes' : 'fault' };
    return { label: item.valid ? '通过' : '失败', status: item.valid ? 'yes' : 'fault' };
  });
}
