// 本文件把网络分区内核状态映射为封闭可视化模式。

import type { MatrixCell, ViewSpec } from '../../../types';
import { chartPattern, graphPattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, matrixCells, metricSeries } from '../networkView';
import { versionGap } from './kernel';
import type { PartitionState } from './model';

/**
 * renderPartitionView 输出分区拓扑、可达性矩阵和风险趋势。
 */
export function renderPartitionView(state: PartitionState): ViewSpec {
  const leftReachable = state.nodes.filter((node) => node.group === 'left' && node.reachable).length;
  const rightReachable = state.nodes.filter((node) => node.group === 'right' && node.reachable).length;
  return {
    summary: `分区${state.partitionActive ? '生效' : '未生效'},左区可达 ${leftReachable},右区可达 ${rightReachable},总可达 ${state.nodes.filter((node) => node.reachable).length}/${state.nodes.length},版本差 ${versionGap(state)}。`,
    patterns: [
      graphPattern('partition-graph', '左右分区拓扑与阻断消息边', graphNodes(state.nodes), graphEdges(state.messages), 'main'),
      matrixPattern('partition-matrix', '分区可达性与版本漂移矩阵', state.nodes.map((node) => node.label), ['区域', '可达', '版本'], partitionCells(state), 'side'),
      chartPattern('partition-chart', '分区风险与同步恢复趋势', metricSeries(state.samples), '%', 'bottom'),
    ],
  };
}

/**
 * partitionCells 展示区域、可达性和版本。
 */
function partitionCells(state: PartitionState): MatrixCell[][] {
  return matrixCells(state.nodes.map((node) => node.label), ['区域', '可达', '版本'], (row, column) => {
    const node = state.nodes.find((item) => item.label === row);
    if (!node) return { label: '无', status: 'empty' };
    if (column === '区域') return { label: node.group === 'left' ? '左区' : '右区', status: 'pending' };
    if (column === '可达') return { label: node.reachable ? '可达' : '阻断', status: node.reachable ? 'yes' : 'fault' };
    return { label: String(node.syncedVersion), status: versionGap(state) === 0 ? 'yes' : 'pending' };
  });
}
