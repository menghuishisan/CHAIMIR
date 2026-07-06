// 本文件把 DHT 路由内核状态映射为封闭可视化模式。

import type { MatrixCell, TeachingFrame } from '../../../types';
import { teachingFrame, graphPattern, matrixPattern, pipelinePattern, selectedOrFrameFocus } from '../../packageTools';
import { graphEdges, graphNodes, matrixCells, pipelineSteps } from '../networkView';
import { distance } from './kernel';
import { dhtPhases, type DhtState } from './model';

/**
 * renderDhtView 输出路由图、异或距离矩阵和查询流程。
 */
export function renderDhtView(state: DhtState): TeachingFrame {
  const closest = state.peers.find((peer) => peer.closest);
    const summary = `查找 key ${state.lookupKey},跳数 ${state.hops},短名单 ${state.shortlist.length} 个,最近节点 ${closest?.label ?? '无'},最近距离 ${closest ? distance(closest, state.lookupKey) : '无'}。`;
  const patterns = [
      graphPattern('dht-graph', 'Kademlia XOR 路由路径', graphNodes(state.peers), graphEdges(state.messages)),
      matrixPattern('dht-matrix', 'XOR 距离排序与候选短名单', state.peers.map((peer) => peer.label), ['节点 ID', 'K 桶', 'XOR 距离', '候选状态'], dhtCells(state)),
      pipelinePattern('dht-pipeline', '选择近邻 -> 查询 -> 收敛到最近节点流程', pipelineSteps([...dhtPhases], state.phaseIndex, state.shortlist.some((id) => state.peers.find((peer) => peer.id === id)?.polluted)), dhtPhases[state.phaseIndex].id),
    ];
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
      primary: selectedOrFrameFocus(state.selectedElementId, ['dht-graph']),
      secondary: ['dht-matrix', 'dht-pipeline'],
    },
    layout: {
      primary: 'dht-graph',
      evidence: ['dht-matrix', 'dht-pipeline'],
    },
    patterns,
  });
}

/**
 * dhtCells 展示 ID、桶、距离和污染状态。
 */
function dhtCells(state: DhtState): MatrixCell[][] {
  return matrixCells(state.peers.map((peer) => peer.label), ['ID', '桶', '距离', '状态'], (row, column) => {
    const peer = state.peers.find((item) => item.label === row);
    if (!peer) return { label: '无', status: 'empty' };
      if (column === '节点 ID') return { label: String(peer.nodeId), status: 'yes' };
      if (column === 'K 桶') return { label: String(peer.bucket), status: peer.inShortlist ? 'yes' : 'pending' };
      if (column === 'XOR 距离') return { label: String(distance(peer, state.lookupKey)), status: peer.closest ? 'yes' : 'pending' };
    return { label: peer.polluted ? '污染' : peer.queried ? '已问' : peer.inShortlist ? '候选' : '备用', status: peer.polluted ? 'fault' : peer.queried ? 'yes' : peer.inShortlist ? 'pending' : 'empty' };
  });
}
