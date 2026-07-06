// 本文件把状态快照状态转换为矩阵、趋势图和流程三种语义可视化。

import type { MatrixCell, TeachingFrame } from '../../../types';
import { teachingFrame, chartPattern, matrixPattern, pipelinePattern, selectedOrFrameFocus } from '../../packageTools';
import { matrixCells, metricSeries, pipelineSteps } from '../dataView';
import { snapshotPhases, type SnapshotState } from './model';

/**
 * renderSnapshotView 基于内核状态生成状态快照可视化。
 */
export function renderSnapshotView(state: SnapshotState): TeachingFrame {
  const dirtyCount = state.accounts.filter((account) => account.dirty).length;
  const restoredCount = state.accounts.filter((account) => account.restored).length;
    const summary = `快照根 ${state.snapshotRoot.slice(0, 8)},当前根 ${state.currentRoot.slice(0, 8)},脏账户 ${dirtyCount},已恢复 ${restoredCount}。`;
  const patterns = [matrixPattern('snapshot-matrix', '账户状态快照与回滚矩阵', state.accounts.map((account) => account.id), ['余额', 'Nonce', '脏写', '恢复'], snapshotCells(state)), chartPattern('snapshot-chart', '状态根一致性趋势', metricSeries(state.samples), '%'), pipelinePattern('snapshot-pipeline', '拍快照 -> 写入 -> 比对根 -> 回滚流程', pipelineSteps(snapshotPhases, state.phaseIndex, state.currentRoot !== state.snapshotRoot && state.phaseIndex >= 4), snapshotPhases[state.phaseIndex].id)];
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
      primary: selectedOrFrameFocus(state.selectedElementId, ['snapshot-chart']),
      secondary: ['snapshot-matrix', 'snapshot-pipeline'],
    },
    layout: {
      primary: 'snapshot-chart',
      evidence: ['snapshot-matrix', 'snapshot-pipeline'],
    },
    patterns,
  });
}

/**
 * snapshotCells 展示账户快照矩阵。
 */
function snapshotCells(state: SnapshotState): MatrixCell[][] {
  return matrixCells(state.accounts.map((account) => account.id), ['余额', 'Nonce', '脏写', '恢复'], (row, column) => {
    const account = state.accounts.find((item) => item.id === row);
    if (!account) return { label: '无', status: 'empty' };
    if (column === '余额') return { label: String(account.balance), status: account.dirty ? 'pending' : 'yes' };
    if (column === 'Nonce') return { label: String(account.nonce), status: 'yes' };
    if (column === '脏写') return { label: account.dirty ? '是' : '否', status: account.dirty ? 'fault' : 'empty' };
    return { label: account.restored ? '已恢复' : '未恢复', status: account.restored ? 'yes' : 'empty' };
  });
}
