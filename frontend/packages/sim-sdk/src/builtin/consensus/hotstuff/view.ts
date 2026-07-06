// 本文件把 HotStuff 内核状态映射为封闭可视化模式。

import type { ChainBlock, TeachingFrame } from '../../../types';
import { teachingFrame, chainPattern, graphPattern, lanePattern, selectedOrFrameFocus } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, type ViewNode } from '../consensusView';
import type { HotStuffBlock, HotStuffState } from './model';
import { labelHotStuffReplica } from './kernel';

/**
 * renderHotStuffView 输出 QC 链、副本网络和消息泳道。
 */
export function renderHotStuffView(state: HotStuffState): TeachingFrame {
  const liveVotes = state.replicas.filter((replica) => replica.voted && !replica.faulty).length;
    const summary = `视图 ${state.view},领导者 ${labelHotStuffReplica(state, state.leaderId)},有效投票 ${liveVotes}/${state.replicas.length},High QC ${state.highQcBlock},锁定块 ${state.lockedBlock},提交块 ${state.committedBlock ?? '未提交'}。`;
  const patterns = [
      chainPattern('hotstuff-chain', `HotStuff 三链提交视图,High QC ${state.highQcBlock}`, hotstuffBlocks(state.blocks), []),
      graphPattern('hotstuff-graph', `HotStuff 领导者提案与投票网络,投票 ${liveVotes}`, replicaNodes(state), graphEdges(state.messages)),
      lanePattern('hotstuff-lane', 'HotStuff propose / vote / QC 时序', state.replicas.map((replica) => replica.label), laneMessages(state.messages, (id) => labelHotStuffReplica(state, id)), state.tick),
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
      primary: selectedOrFrameFocus(state.selectedElementId, ['hotstuff-lane']),
      secondary: ['hotstuff-chain', 'hotstuff-graph'],
    },
    layout: {
      primary: 'hotstuff-lane',
      evidence: ['hotstuff-chain', 'hotstuff-graph'],
    },
    patterns,
  });
}

/**
 * replicaNodes 把 HotStuff 副本状态映射为图节点。
 */
function replicaNodes(state: HotStuffState): ViewNode[] {
  return graphNodes(state.replicas.map((replica) => ({ id: replica.id, label: replica.label, role: 'hotstuff-replica', status: replica.faulty ? 'danger' : replica.leader ? 'active' : replica.voted ? 'success' : replica.timeout ? 'warning' : 'idle', value: replica.leader ? '领导者' : `锁 ${replica.lockedBlock}` })));
}

/**
 * hotstuffBlocks 将 HotStuff 区块转换成链式 QC 可视化结构。
 */
function hotstuffBlocks(blocks: HotStuffBlock[]): ChainBlock[] {
  return blocks.map((block, index) => ({ id: block.id, height: index, hash: block.hash, parentHash: block.parentId ?? '', label: block.view === 0 ? '创世块' : block.committed ? `提交 v${block.view}` : block.qc ? `QC v${block.view}` : `提案 v${block.view}`, status: block.committed ? 'canonical' : block.qc ? 'pending' : 'orphaned' }));
}
