// 本文件把闪电贷攻击状态转换为组合调用图、原子时序和风控矩阵。

import type { MatrixCell, ViewSpec } from '../../../types';
import { graphPattern, lanePattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, matrixCells } from '../securityView';
import type { FlashLoanState } from './model';

/**
 * renderFlashLoanView 基于内核状态生成闪电贷组合攻击可视化。
 */
export function renderFlashLoanView(state: FlashLoanState): ViewSpec {
  return { summary: `借款 ${state.loanAmount},池价 ${state.poolPrice},利润 ${state.attackerProfit},限额${state.limitEnabled ? '已启用' : '未启用'}。`, patterns: [graphPattern('flash-graph', '闪电贷组合调用', graphNodes(state.actors), graphEdges(state.calls), 'main'), lanePattern('flash-lane', '原子交易时序', state.actors.map((actor) => actor.label), laneMessages(state.calls, (id) => labelOf(state, id)), state.tick, 'side'), matrixPattern('flash-matrix', '风险控制', ['借款限额', '价格保护', '冷却时间'], ['结果'], flashCells(state), 'bottom')] };
}

/**
 * flashCells 展示防护措施状态。
 */
function flashCells(state: FlashLoanState): MatrixCell[][] {
  return matrixCells(['借款限额', '价格保护', '冷却时间'], ['结果'], () => ({ label: state.limitEnabled ? '已启用' : '未启用', status: state.limitEnabled ? 'yes' : 'fault' }));
}

/**
 * labelOf 返回参与方展示名称。
 */
function labelOf(state: FlashLoanState, id: string): string {
  return state.actors.find((actor) => actor.id === id)?.label ?? id;
}
