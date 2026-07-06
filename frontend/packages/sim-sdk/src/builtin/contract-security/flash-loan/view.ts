// 本文件把闪电贷攻击状态转换为组合调用图、原子时序和风控矩阵。

import type { MatrixCell, ViewSpec } from '../../../types';
import { graphPattern, lanePattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, matrixCells } from '../securityView';
import type { FlashLoanState } from './model';

/**
 * renderFlashLoanView 基于内核状态生成闪电贷组合攻击可视化。
 */
export function renderFlashLoanView(state: FlashLoanState): ViewSpec {
  const priceShift = state.poolPrice - state.basePoolPrice;
  const debtGap = Math.max(0, state.protocolDebt - state.loanAmount);
  return { summary: `借款 ${state.loanAmount},池价偏移 ${priceShift},协议债务缺口 ${debtGap},攻击利润 ${state.attackerProfit},限额${state.limitEnabled ? '已启用' : '未启用'}。`, patterns: [graphPattern('flash-graph', '闪电贷借款 -> 操纵 -> 获利调用图', graphNodes(state.actors), graphEdges(state.calls), 'main'), lanePattern('flash-lane', '单笔原子交易内的组合调用时序', state.actors.map((actor) => actor.label), laneMessages(state.calls, (id) => labelOf(state, id)), state.tick, 'side'), matrixPattern('flash-matrix', '闪电贷风控断点', ['单笔借款限额', '价格偏移保护', '冷却/延迟窗口'], ['结果'], flashCells(state), 'bottom')] };
}

/**
 * flashCells 展示防护措施状态。
 */
function flashCells(state: FlashLoanState): MatrixCell[][] {
  const priceShift = Math.abs(state.poolPrice - state.basePoolPrice);
  return matrixCells(['单笔借款限额', '价格偏移保护', '冷却/延迟窗口'], ['结果'], (row) => {
    if (row === '单笔借款限额') return { label: state.limitEnabled ? `限制 ${state.baseLoanAmount}` : `暴露 ${state.loanAmount}`, status: state.limitEnabled ? 'yes' : 'fault' };
    if (row === '价格偏移保护') return { label: state.limitEnabled ? `偏移 ${priceShift}` : '未校验', status: state.limitEnabled ? 'yes' : 'fault' };
    return { label: state.containedAttempt ? '已阻断' : state.limitEnabled ? '等待观察' : '无延迟', status: state.containedAttempt ? 'yes' : state.limitEnabled ? 'pending' : 'fault' };
  });
}

/**
 * labelOf 返回参与方展示名称。
 */
function labelOf(state: FlashLoanState, id: string): string {
  return state.actors.find((actor) => actor.id === id)?.label ?? id;
}
