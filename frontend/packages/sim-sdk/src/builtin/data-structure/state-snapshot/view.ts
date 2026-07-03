// 本文件把状态快照状态转换为矩阵、趋势图和流程三种语义可视化。

import type { MatrixCell, ViewSpec } from '../../../types';
import { chartPattern, matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, metricSeries, pipelineSteps } from '../dataView';
import { snapshotPhases, type SnapshotState } from './model';

/**
 * renderSnapshotView 基于内核状态生成状态快照可视化。
 */
export function renderSnapshotView(state: SnapshotState): ViewSpec {
  return { summary: `快照根 ${state.snapshotRoot.slice(0, 8)},当前根 ${state.currentRoot.slice(0, 8)},脏账户 ${state.accounts.filter((account) => account.dirty).length} 个。`, patterns: [matrixPattern('snapshot-matrix', '账户快照状态', state.accounts.map((account) => account.id), ['余额', 'nonce', '脏写', '恢复'], snapshotCells(state), 'main'), chartPattern('snapshot-chart', '快照一致性趋势', metricSeries(state.samples), '%', 'side'), pipelinePattern('snapshot-pipeline', '快照流程', pipelineSteps(snapshotPhases, state.phaseIndex, state.currentRoot !== state.snapshotRoot && state.phaseIndex >= 4), snapshotPhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * snapshotCells 展示账户快照矩阵。
 */
function snapshotCells(state: SnapshotState): MatrixCell[][] {
  return matrixCells(state.accounts.map((account) => account.id), ['余额', 'nonce', '脏写', '恢复'], (row, column) => {
    const account = state.accounts.find((item) => item.id === row);
    if (!account) return { label: '无', status: 'empty' };
    if (column === '余额') return { label: String(account.balance), status: account.dirty ? 'pending' : 'yes' };
    if (column === 'nonce') return { label: String(account.nonce), status: 'yes' };
    if (column === '脏写') return { label: account.dirty ? '是' : '否', status: account.dirty ? 'fault' : 'empty' };
    return { label: account.restored ? '已恢复' : '未恢复', status: account.restored ? 'yes' : 'empty' };
  });
}
