// 本文件把 UTXO 集合状态转换为矩阵和流程两种语义可视化。

import type { MatrixCell, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../dataView';
import { utxoPhases, type UtxoState } from './model';

/**
 * renderUtxoView 基于内核状态生成 UTXO 集合可视化。
 */
export function renderUtxoView(state: UtxoState): ViewSpec {
  const inputValue = state.inputs.reduce((sum, inputId) => sum + (state.utxos.find((utxo) => utxo.id === inputId)?.amount ?? 0), 0);
  const outputValue = state.outputs.reduce((sum, output) => sum + output.amount, 0);
  return { summary: `输入 ${state.inputs.length} 个/${inputValue},输出 ${state.outputs.length} 个/${outputValue},找零 ${Math.max(0, inputValue - outputValue)},交易${state.txValid ? '有效' : '待校验'}。`, patterns: [matrixPattern('utxo-matrix', 'UTXO 输入选择与双花检测矩阵', state.utxos.map((utxo) => utxo.id), ['所有者', '金额', '花费状态', '双花'], utxoCells(state), 'main'), pipelinePattern('utxo-pipeline', '选择输入 -> 检查双花 -> 守恒输出流程', pipelineSteps(utxoPhases, state.phaseIndex, state.utxos.some((utxo) => utxo.doubleSpend)), utxoPhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * utxoCells 展示 UTXO 状态矩阵。
 */
function utxoCells(state: UtxoState): MatrixCell[][] {
  return matrixCells(state.utxos.map((item) => item.id), ['所有者', '金额', '花费状态', '双花'], (row, column) => {
    const item = state.utxos.find((entry) => entry.id === row);
    if (!item) return { label: '无', status: 'empty' };
    if (column === '所有者') return { label: item.owner, status: 'yes' };
    if (column === '金额') return { label: String(item.amount), status: 'yes' };
    if (column === '花费状态') return { label: item.spent ? '已花费' : item.selected ? '已选' : '未花费', status: item.spent ? 'pending' : item.selected ? 'yes' : 'empty' };
    return { label: item.doubleSpend ? '是' : '否', status: item.doubleSpend ? 'fault' : 'empty' };
  });
}
