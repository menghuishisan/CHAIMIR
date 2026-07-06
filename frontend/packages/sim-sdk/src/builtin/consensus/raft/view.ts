// 本文件把 Raft 内核状态映射为封闭可视化模式,不包含状态迁移。

import type { MatrixCell, TeachingFrame } from '../../../types';
import { teachingFrame, graphPattern, lanePattern, matrixPattern, selectedOrFrameFocus } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, voteCells, type ViewNode } from '../consensusView';
import { labelRaftNode, quorum, replicatedCount } from './kernel';
import type { RaftState } from './model';

/**
 * renderRaftView 输出角色图、RPC 泳道和日志复制矩阵。
 */
export function renderRaftView(state: RaftState): TeachingFrame {
  const copied = replicatedCount(state);
  const required = quorum(state);
  const partitioned = state.nodes.filter((node) => node.partitioned).length;
    const summary = `任期 ${state.term},领导者 ${labelRaftNode(state, state.leaderId)},提交索引 ${state.commitIndex},日志复制 ${copied}/${required},分区节点 ${partitioned}。`;
  const patterns = [
      graphPattern('raft-graph', `Raft 领导者与多数派,复制 ${copied}/${required}`, raftNodes(state), graphEdges(state.messages)),
      lanePattern('raft-lane', 'Raft RequestVote / AppendEntries 时序', state.nodes.map((node) => node.label), laneMessages(state.messages, (id) => labelRaftNode(state, id)), state.tick),
      matrixPattern('raft-matrix', '日志匹配与提交矩阵', state.nodes.map((node) => node.label), ['任期', '角色', '日志匹配', 'nextIndex', '提交/应用'], raftCells(state)),
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
      primary: selectedOrFrameFocus(state.selectedElementId, ['raft-matrix']),
      secondary: ['raft-graph', 'raft-lane'],
    },
    layout: {
      primary: 'raft-matrix',
      evidence: ['raft-graph'],
      timeline: 'raft-lane',
    },
    patterns,
  });
}

/**
 * raftNodes 将 Raft 角色和分区状态映射为图节点。
 */
function raftNodes(state: RaftState): ViewNode[] {
  return graphNodes(state.nodes.map((node) => ({ id: node.id, label: node.label, role: 'raft-node', status: node.partitioned ? 'danger' : node.role === 'leader' ? 'active' : node.matchIndex >= state.commitIndex && state.commitIndex > 0 ? 'success' : node.role === 'candidate' ? 'warning' : 'idle', value: node.role === 'leader' ? '领导者' : node.role === 'candidate' ? '候选者' : '跟随者' })));
}

/**
 * raftCells 展示节点任期、角色、日志和提交状态。
 */
function raftCells(state: RaftState): MatrixCell[][] {
  return voteCells(
    state.nodes.map((node) => node.label),
    ['任期', '角色', '日志', 'nextIndex', '提交/应用'],
    (row, column) => {
      const node = state.nodes.find((item) => item.label === row);
      if (!node) return { label: '无', status: 'empty' };
      if (node.partitioned) return { label: '分区', status: 'fault' };
      if (column === '任期') return { label: String(node.term), status: 'yes' };
      if (column === '角色') return { label: node.role === 'leader' ? '领导' : node.role === 'candidate' ? '竞选' : '跟随', status: node.role === 'leader' ? 'yes' : 'pending' };
      if (column === '日志匹配') return { label: `${node.logLength}/${state.log.length}`, status: node.logLength === state.log.length ? 'yes' : 'pending' };
      if (column === 'nextIndex') return { label: String(node.nextIndex), status: node.nextIndex === state.log.length + 1 ? 'yes' : 'pending' };
      return { label: `${node.commitIndex}/${node.appliedIndex}`, status: node.appliedIndex >= state.commitIndex && state.commitIndex > 0 ? 'yes' : 'empty' };
    }
  );
}
