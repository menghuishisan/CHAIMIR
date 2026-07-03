// 本文件把 DHT 路由内核状态映射为封闭可视化模式。

import type { MatrixCell, ViewSpec } from '../../../types';
import { graphPattern, matrixPattern, pipelinePattern } from '../../packageTools';
import { graphEdges, graphNodes, matrixCells, pipelineSteps } from '../networkView';
import { distance } from './kernel';
import { dhtPhases, type DhtState } from './model';

/**
 * renderDhtView 输出路由图、异或距离矩阵和查询流程。
 */
export function renderDhtView(state: DhtState): ViewSpec {
  return {
    summary: `查找 key ${state.lookupKey},跳数 ${state.hops},短名单 ${state.shortlist.length} 个,最近节点 ${state.peers.find((peer) => peer.closest)?.label ?? '无'}。`,
    patterns: [
      graphPattern('dht-graph', 'DHT 路由路径', graphNodes(state.peers), graphEdges(state.messages), 'main'),
      matrixPattern('dht-matrix', '异或距离表', state.peers.map((peer) => peer.label), ['ID', '桶', '距离', '状态'], dhtCells(state), 'side'),
      pipelinePattern('dht-pipeline', 'DHT 查询流程', pipelineSteps([...dhtPhases], state.phaseIndex, state.shortlist.some((id) => state.peers.find((peer) => peer.id === id)?.polluted)), dhtPhases[state.phaseIndex].id, 'bottom'),
    ],
  };
}

/**
 * dhtCells 展示 ID、桶、距离和污染状态。
 */
function dhtCells(state: DhtState): MatrixCell[][] {
  return matrixCells(state.peers.map((peer) => peer.label), ['ID', '桶', '距离', '状态'], (row, column) => {
    const peer = state.peers.find((item) => item.label === row);
    if (!peer) return { label: '无', status: 'empty' };
    if (column === 'ID') return { label: String(peer.nodeId), status: 'yes' };
    if (column === '桶') return { label: String(peer.bucket), status: peer.inShortlist ? 'yes' : 'pending' };
    if (column === '距离') return { label: String(distance(peer, state.lookupKey)), status: peer.closest ? 'yes' : 'pending' };
    return { label: peer.polluted ? '污染' : peer.queried ? '已问' : peer.inShortlist ? '候选' : '备用', status: peer.polluted ? 'fault' : peer.queried ? 'yes' : peer.inShortlist ? 'pending' : 'empty' };
  });
}
