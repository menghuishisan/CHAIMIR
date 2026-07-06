// 本文件把 P2P 发现内核状态映射为封闭可视化模式。

import type { MatrixCell, ViewSpec } from '../../../types';
import { graphPattern, matrixPattern, pipelinePattern } from '../../packageTools';
import { graphEdges, graphNodes, matrixCells, pipelineSteps } from '../networkView';
import { discoveryPhases, type DiscoveryState } from './model';

/**
 * renderDiscoveryView 输出发现拓扑、连接健康矩阵和发现流程。
 */
export function renderDiscoveryView(state: DiscoveryState): ViewSpec {
  const failedHandshake = state.peers.filter((peer) => peer.failedHandshakeReason).length;
  return {
    summary: `已连接 ${state.peers.filter((peer) => peer.connected).length} 个节点,握手 ${state.handshakeCount} 次,失败握手 ${failedHandshake},黑名单 ${state.bannedPeerIds.length} 个。`,
    patterns: [
      graphPattern('discovery-graph', 'P2P 地址发现与握手拓扑', graphNodes(state.peers), graphEdges(state.messages), 'main'),
      matrixPattern('discovery-matrix', '地址簿、握手健康与黑名单矩阵', state.peers.map((peer) => peer.label), ['地址数量', '握手', '健康', '黑名单'], discoveryCells(state), 'side'),
      pipelinePattern('discovery-pipeline', '发现地址 -> 握手 -> 健康过滤 -> 剔除流程', pipelineSteps([...discoveryPhases], state.phaseIndex, state.peers.some((peer) => (peer.malicious || peer.failedHandshakeReason) && !peer.banned)), discoveryPhases[state.phaseIndex].id, 'bottom'),
    ],
  };
}

/**
 * discoveryCells 展示地址、握手、健康和黑名单状态。
 */
function discoveryCells(state: DiscoveryState): MatrixCell[][] {
  return matrixCells(state.peers.map((peer) => peer.label), ['地址', '握手', '健康', '黑名单'], (row, column) => {
    const peer = state.peers.find((item) => item.label === row);
    if (!peer) return { label: '无', status: 'empty' };
      if (column === '地址数量') return { label: String(peer.knownAddrs.length || state.addressBook.filter((address) => address.peerId === peer.id).length), status: peer.malicious ? 'fault' : 'yes' };
    if (column === '握手') return { label: peer.failedHandshakeReason ?? (peer.connected ? '通过' : '等待'), status: peer.failedHandshakeReason ? 'fault' : peer.connected ? 'yes' : 'pending' };
    if (column === '健康') return { label: peer.healthy ? '正常' : '异常', status: peer.healthy ? 'yes' : 'fault' };
    return { label: peer.banned ? '已剔除' : '否', status: peer.banned ? 'fault' : 'empty' };
  });
}
