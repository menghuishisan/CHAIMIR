// 本文件把重入攻击状态转换为调用图、调用栈泳道和安全矩阵。

import type { MatrixCell, ViewSpec } from '../../../types';
import { graphPattern, lanePattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, matrixCells } from '../securityView';
import type { ReentrancyState } from './model';

/**
 * renderReentrancyView 基于内核状态生成重入攻击过程可视化。
 */
export function renderReentrancyView(state: ReentrancyState): ViewSpec {
  return { summary: `金库余额 ${state.vaultBalance},攻击者余额 ${state.attackerBalance},重入锁${state.lockEnabled ? '已启用' : '未启用'}。`, patterns: [graphPattern('reentrancy-graph', '调用与资金流', graphNodes(state.actors), graphEdges(state.calls), 'main'), lanePattern('reentrancy-lane', '重入调用栈', state.actors.map((actor) => actor.label), laneMessages(state.calls, (id) => labelOf(state, id)), state.tick, 'side'), matrixPattern('reentrancy-matrix', '安全条件', ['余额先改', '重入锁', '外部调用'], ['结果'], reentrancyCells(state), 'bottom')] };
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
