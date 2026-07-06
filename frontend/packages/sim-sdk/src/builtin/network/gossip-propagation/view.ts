// 本文件把 Gossip 传播内核状态映射为封闭可视化模式。

import type { MatrixCell, ViewSpec } from '../../../types';
import { chartPattern, graphPattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, matrixCells, metricSeries } from '../networkView';
import { coverage } from './kernel';
import type { GossipState } from './model';

/**
 * renderGossipView 输出传播拓扑、节点接收矩阵和趋势图。
 */
export function renderGossipView(state: GossipState): ViewSpec {
  const duplicates = state.peers.reduce((sum, peer) => sum + peer.duplicateCount, 0);
  return {
    summary: `覆盖率 ${coverage(state)}%,扇出 ${state.fanout},轮次 ${state.round},重复消息 ${duplicates},污染节点 ${state.peers.filter((peer) => peer.polluted).length}。`,
    patterns: [
      graphPattern('gossip-graph', `Gossip 扇出传播拓扑,fanout=${state.fanout}`, graphNodes(state.peers), graphEdges(state.messages), 'main'),
      matrixPattern('gossip-matrix', '节点接收、重复与污染传播矩阵', state.peers.map((peer) => peer.label), ['收到', '重复消息', '污染状态'], gossipCells(state), 'side'),
      chartPattern('gossip-chart', '覆盖率 / 风险 / 延迟传播趋势', metricSeries(state.samples), '%', 'bottom'),
    ],
  };
}

/**
 * gossipCells 展示节点传播状态。
 */
function gossipCells(state: GossipState): MatrixCell[][] {
  return matrixCells(state.peers.map((peer) => peer.label), ['收到', '重复', '污染'], (row, column) => {
    const peer = state.peers.find((item) => item.label === row);
    if (!peer) return { label: '无', status: 'empty' };
    if (column === '收到') return { label: peer.informed ? '是' : '否', status: peer.informed ? 'yes' : 'empty' };
      if (column === '重复消息') return { label: String(peer.duplicateCount), status: peer.duplicateCount > 0 ? 'pending' : 'empty' };
    return { label: peer.polluted ? '污染' : '正常', status: peer.polluted ? 'fault' : 'yes' };
  });
}
