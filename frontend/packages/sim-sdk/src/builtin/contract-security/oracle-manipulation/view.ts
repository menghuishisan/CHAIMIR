// 本文件把预言机操纵状态转换为价格源图、检查矩阵和价格趋势。

import type { MatrixCell, ViewSpec } from '../../../types';
import { chartPattern, graphPattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, matrixCells } from '../securityView';
import type { OracleState } from './model';

/**
 * renderOracleView 基于内核状态生成预言机风险可视化。
 */
export function renderOracleView(state: OracleState): ViewSpec {
  const spotDeviation = state.spotPrice - state.referencePrice;
  const twapDeviation = state.twapPrice - state.referencePrice;
  return { summary: `参考价 ${state.referencePrice},现货偏离 ${spotDeviation},TWAP 偏离 ${twapDeviation},操纵${state.manipulationActive ? '进行中' : '未生效'}。`, patterns: [graphPattern('oracle-graph', '价格源 -> 聚合器 -> 借贷合约信任路径', graphNodes(state.actors), graphEdges(state.calls), 'main'), matrixPattern('oracle-matrix', '预言机偏离阈值检查', ['现货价', 'TWAP', '多源中位数'], ['价格', '偏离', '状态'], oracleCells(state), 'side'), chartPattern('oracle-chart', '现货 / TWAP / 参考价偏离', [{ label: '价格', points: [{ x: 0, y: state.referencePrice }, { x: 1, y: state.spotPrice }, { x: 2, y: state.twapPrice }] }], '价格', 'bottom')] };
}

/**
 * oracleCells 展示价格源可信状态。
 */
function oracleCells(state: OracleState): MatrixCell[][] {
  return matrixCells(['现货价', 'TWAP', '多源中位数'], ['价格', '偏离', '状态'], (row, column) => {
    const value = row === '现货价' ? state.spotPrice : row === 'TWAP' ? state.twapPrice : state.referencePrice;
    if (column === '价格') return { label: String(value), status: 'yes' };
    if (column === '偏离') return { label: String(value - state.referencePrice), status: Math.abs(value - state.referencePrice) <= 5 ? 'yes' : 'fault' };
    return { label: Math.abs(value - state.referencePrice) <= 5 ? '可信' : '偏离', status: Math.abs(value - state.referencePrice) <= 5 ? 'yes' : 'fault' };
  });
}
