// 本文件把预言机操纵状态转换为价格源图、检查矩阵和价格趋势。

import type { MatrixCell, ViewSpec } from '../../../types';
import { chartPattern, graphPattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, matrixCells } from '../securityView';
import type { OracleState } from './model';

/**
 * renderOracleView 基于内核状态生成预言机风险可视化。
 */
export function renderOracleView(state: OracleState): ViewSpec {
  return { summary: `现货价 ${state.spotPrice},TWAP ${state.twapPrice},参考价 ${state.referencePrice}。`, patterns: [graphPattern('oracle-graph', '价格源与借贷合约', graphNodes(state.actors), graphEdges(state.calls), 'main'), matrixPattern('oracle-matrix', '价格检查', ['现货价', 'TWAP', '多源中位数'], ['价格', '状态'], oracleCells(state), 'side'), chartPattern('oracle-chart', '价格偏离', [{ label: '价格', points: [{ x: 0, y: state.referencePrice }, { x: 1, y: state.spotPrice }, { x: 2, y: state.twapPrice }] }], 'price', 'bottom')] };
}

/**
 * oracleCells 展示价格源可信状态。
 */
function oracleCells(state: OracleState): MatrixCell[][] {
  return matrixCells(['现货价', 'TWAP', '多源中位数'], ['价格', '状态'], (row, column) => {
    const value = row === '现货价' ? state.spotPrice : row === 'TWAP' ? state.twapPrice : state.referencePrice;
    if (column === '价格') return { label: String(value), status: 'yes' };
    return { label: Math.abs(value - state.referencePrice) <= 5 ? '可信' : '偏离', status: Math.abs(value - state.referencePrice) <= 5 ? 'yes' : 'fault' };
  });
}
