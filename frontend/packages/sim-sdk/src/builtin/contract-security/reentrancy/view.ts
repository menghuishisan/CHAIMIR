// 本文件把重入攻击状态转换为调用图、调用栈泳道和安全矩阵。

import type { MatrixCell, TeachingFrame } from '../../../types';
import { teachingFrame, graphPattern, lanePattern, matrixPattern, selectedOrFrameFocus } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, matrixCells } from '../securityView';
import type { ReentrancyState } from './model';

/**
 * renderReentrancyView 基于内核状态生成重入攻击过程可视化。
 */
export function renderReentrancyView(state: ReentrancyState): TeachingFrame {
  const totalAssets = state.vaultBalance + state.attackerBalance;
    const summary = `金库余额 ${state.vaultBalance},攻击者余额 ${state.attackerBalance},资产观察值 ${totalAssets},重入${state.reentered ? '已发生' : '未发生'},锁${state.lockEnabled ? '已启用' : '未启用'}。`;
  const patterns = [graphPattern('reentrancy-graph', '外部调用前后资金流与回调边', graphNodes(state.actors), graphEdges(state.calls)), lanePattern('reentrancy-lane', '重入调用栈深度时序', state.actors.map((actor) => actor.label), laneMessages(state.calls, (id) => labelOf(state, id)), state.tick), matrixPattern('reentrancy-matrix', 'Checks-Effects-Interactions 防护矩阵', ['余额先改', '重入锁', '外部调用'], ['结果'], reentrancyCells(state))];
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
      primary: selectedOrFrameFocus(state.selectedElementId, ['reentrancy-graph']),
      secondary: ['reentrancy-lane', 'reentrancy-matrix'],
    },
    layout: {
      primary: 'reentrancy-graph',
      evidence: ['reentrancy-matrix'],
      timeline: 'reentrancy-lane',
    },
    patterns,
  });
}

/**
 * reentrancyCells 展示重入防护关键条件。
 */
function reentrancyCells(state: ReentrancyState): MatrixCell[][] {
  return matrixCells(['余额先改', '重入锁', '外部调用'], ['结果'], (row) => {
    if (row === '重入锁') return { label: state.lockEnabled ? '已启用' : '未启用', status: state.lockEnabled ? 'yes' : 'fault' };
    if (row === '余额先改') return { label: state.lockEnabled ? '是' : '否', status: state.lockEnabled ? 'yes' : 'fault' };
    return { label: state.reentered ? '被重入' : '受控', status: state.reentered ? 'fault' : 'yes' };
  });
}

/**
 * labelOf 返回参与方展示名称。
 */
function labelOf(state: ReentrancyState, id: string): string {
  return state.actors.find((actor) => actor.id === id)?.label ?? id;
}
