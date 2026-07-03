// 本文件实现 Raft 选举与日志复制内核,所有动画和检查点都从该状态机派生。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { indexFromSeed, integerParam, stringParam } from '../../initParams';
import { majorityThreshold } from '../consensusPrimitives';
import { processViewMessage, refreshViewMessages, type ViewMessage } from '../consensusView';
import { raftPhases, type RaftNode, type RaftState } from './model';
import { traceLinesForRaft } from './trace';

/**
 * createInitialRaftState 根据初始化参数创建 Raft 集群、任期和首条日志。
 */
export function createInitialRaftState(params: SimInitParams, seed: number): RaftState {
  const nodeCount = integerParam(params, 'nodeCount', 5, 3, 9);
  const term = integerParam(params, 'term', 1, 1, 1000);
  const leaderIndex = integerParam(params, 'leaderIndex', indexFromSeed(seed, nodeCount) + 1, 1, nodeCount) - 1;
  const command = stringParam(params, 'command', 'set course=consensus', 96);
  const nodes = Array.from({ length: nodeCount }, (_, index): RaftNode => {
    const label = `N${index + 1}`;
    const leaderId = `raft-n${leaderIndex + 1}`;
    return { id: `raft-n${index + 1}`, label, role: index === leaderIndex ? 'leader' : 'follower', term, votedFor: index === leaderIndex ? undefined : leaderId, logLength: 1, lastLogTerm: term, matchIndex: index === leaderIndex ? 1 : 0, nextIndex: 2, commitIndex: 0, appliedIndex: 0, partitioned: false };
  });
  const leaderId = nodes[leaderIndex].id;
  return finalizeRaftState({
    tick: 0,
    phase: raftPhases[0].label,
    phaseIndex: 0,
    term,
    commitIndex: 0,
    leaderId,
    nodes,
    log: [{ index: 1, term, command, committed: false }],
    messages: [],
    votes: {},
    partitionActive: false,
    lastTransition: raftPhases[0].id,
    explanation: explainRaftPhase(0),
    metrics: {},
    checkpointValues: {},
  });
}

/**
 * reduceRaftEvent 是 Raft 仿真包唯一事件入口,保持回放确定性。
 */
export function reduceRaftEvent(state: RaftState, event: SimEvent, _context: ReducerContext): RaftState {
  if (event.type === 'select') return finalizeRaftState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeRaftState(partitionLeader(state));
  if (event.type === 'recover') return finalizeRaftState(recoverPartition(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeRaftState(advanceRaft(state));
  return state;
}

/**
 * advanceRaft 按 Raft 协议顺序推进一个过程单元。
 */
export function advanceRaft(state: RaftState): RaftState {
  const phaseIndex = Math.min(raftPhases.length - 1, state.phaseIndex + (state.lastTransition === raftPhases[state.phaseIndex].id ? 1 : 0));
  const next = { ...state, phaseIndex, tick: state.tick + 1 };
  if (phaseIndex === 1) return startElection(next);
  if (phaseIndex === 2) return becomeLeader(collectVotes(next));
  if (phaseIndex === 3) return appendEntry(next);
  if (phaseIndex === 4) return replicateEntry(next);
  if (phaseIndex === 5) return commitEntry(next);
  return next;
}

/**
 * raftMajorityCommit 检查提交索引是否由多数派复制支撑。
 */
export function raftMajorityCommit(state: RaftState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.majorityCommit);
  return { achieved, answer: { replicated: replicatedCount(state), quorum: quorum(state), commitIndex: state.commitIndex }, explanation: achieved ? '日志已复制到多数派并安全提交。' : '日志尚未达到多数派提交条件。' };
}

/**
 * raftSingleLeader 检查同一任期是否只有一个领导者。
 */
export function raftSingleLeader(state: RaftState): CheckpointResult {
  const leaders = state.nodes.filter((node) => node.role === 'leader');
  return { achieved: leaders.length === 1, answer: { leaders: leaders.map((node) => node.label), term: state.term }, explanation: leaders.length === 1 ? '当前任期只有一个领导者。' : '领导者数量异常,需要重新选举。' };
}

/**
 * finalizeRaftState 刷新派生说明、指标、消息进度和代码追踪。
 */
export function finalizeRaftState(state: RaftState): RaftState {
  return {
    ...state,
    phase: raftPhases[state.phaseIndex].label,
    explanation: explainRaftPhase(state.phaseIndex),
    messages: refreshViewMessages(state.messages, state.tick, (message) => message.detail ?? `${message.label} RPC 正在传播或等待确认。`),
    metrics: { result: state.commitIndex === state.log.length ? '日志已提交' : '等待多数派', risk: state.partitionActive ? 70 : 12, term: state.term, commitIndex: state.commitIndex },
    checkpointValues: { majorityCommit: state.commitIndex === state.log.length && replicatedCount(state) >= quorum(state), singleLeader: state.nodes.filter((node) => node.role === 'leader').length === 1 },
    _trace: { triggeredLines: traceLinesForRaft(state.lastTransition), variables: { term: state.term, commitIndex: state.commitIndex, leaderId: state.leaderId, lastLogIndex: state.log.length, appliedNodes: state.nodes.filter((node) => node.appliedIndex >= state.commitIndex && state.commitIndex > 0).length }, executionPath: `raft/${state.lastTransition}` },
  };
}

/**
 * startElection 将一个跟随者提升为候选者并广播 RequestVote。
 */
function startElection(state: RaftState): RaftState {
  const candidate = state.nodes.find((node) => !node.partitioned && node.id !== state.leaderId && nodeLogUpToDate(node, lastEntry(state).index, lastEntry(state).term)) ?? state.nodes.find((node) => !node.partitioned) ?? state.nodes[0];
  const term = state.term + 1;
  return { ...state, lastTransition: 'request-vote', term, candidateId: candidate.id, votes: { [candidate.id]: true }, nodes: state.nodes.map((node) => (node.id === candidate.id ? { ...node, role: 'candidate', term, votedFor: candidate.id } : { ...node, term, votedFor: undefined })), messages: state.messages.concat(rpcFrom(state, candidate.id, 'RequestVote')) };
}

/**
 * collectVotes 根据日志新旧和网络分区收集多数派投票。
 */
function collectVotes(state: RaftState): RaftState {
  const candidateId = state.candidateId ?? state.leaderId;
  const candidate = state.nodes.find((node) => node.id === candidateId);
  const votes = { ...state.votes };
  for (const node of state.nodes) {
    if (node.partitioned || node.id === candidateId || !candidate) continue;
    const logOk = nodeLogUpToDate(candidate, node.logLength, node.lastLogTerm);
    const termOk = node.term <= state.term;
    const voteFree = node.votedFor === undefined || node.votedFor === candidateId;
    votes[node.id] = termOk && logOk && voteFree;
  }
  return { ...state, lastTransition: 'win-election', votes, nodes: state.nodes.map((node) => (votes[node.id] ? { ...node, votedFor: candidateId } : node)), messages: state.messages.concat(voteReplies(state, candidateId)) };
}

/**
 * becomeLeader 在获得多数票后设置新领导者。
 */
function becomeLeader(state: RaftState): RaftState {
  const candidateId = state.candidateId ?? state.leaderId;
  if (yesVotes(state) < quorum(state)) return state;
  const nextIndex = state.log.length + 1;
  return { ...state, lastTransition: 'win-election', leaderId: candidateId, nodes: state.nodes.map((node) => ({ ...node, role: node.id === candidateId ? 'leader' : 'follower', votedFor: node.id === candidateId ? undefined : candidateId, nextIndex })) };
}

/**
 * appendEntry 由领导者追加新命令并发送 AppendEntries。
 */
function appendEntry(state: RaftState): RaftState {
  const nextIndex = state.log.length + 1;
  const entry = { index: nextIndex, term: state.term, command: `apply-${nextIndex}`, committed: false };
  return { ...state, lastTransition: 'append-entry', log: state.log.concat(entry), nodes: state.nodes.map((node) => (node.id === state.leaderId ? { ...node, logLength: nextIndex, lastLogTerm: state.term, matchIndex: nextIndex, nextIndex: nextIndex + 1 } : node)), messages: state.messages.concat(rpcFrom(state, state.leaderId, 'AppendEntries')) };
}

/**
 * replicateEntry 更新可达跟随者的日志长度和 matchIndex。
 */
function replicateEntry(state: RaftState): RaftState {
  const leaderLogLength = state.log.length;
  const leaderLastTerm = lastEntry(state).term;
  return {
    ...state,
    lastTransition: 'replicate',
    nodes: state.nodes.map((node) => {
      if (node.partitioned || node.id === state.leaderId) return node;
      const prevIndex = Math.max(0, node.nextIndex - 1);
      const prevMatches = prevIndex === 0 || (node.logLength >= prevIndex && node.lastLogTerm <= leaderLastTerm);
      if (!prevMatches) return { ...node, nextIndex: Math.max(1, node.nextIndex - 1), matchIndex: Math.min(node.matchIndex, node.nextIndex - 2) };
      return { ...node, logLength: leaderLogLength, lastLogTerm: leaderLastTerm, matchIndex: leaderLogLength, nextIndex: leaderLogLength + 1 };
    }),
  };
}

/**
 * commitEntry 在多数派复制后推进提交索引。
 */
function commitEntry(state: RaftState): RaftState {
  const commitIndex = replicatedCount(state) >= quorum(state) ? state.log.length : state.commitIndex;
  return {
    ...state,
    lastTransition: 'commit',
    commitIndex,
    log: state.log.map((entry) => ({ ...entry, committed: entry.index <= commitIndex })),
    nodes: state.nodes.map((node) => {
      if (node.partitioned || node.matchIndex < commitIndex) return node;
      return { ...node, commitIndex, appliedIndex: commitIndex };
    }),
  };
}

/**
 * partitionLeader 注入领导者网络分区。
 */
function partitionLeader(state: RaftState): RaftState {
  return { ...state, tick: state.tick + 1, lastTransition: 'timeout', partitionActive: true, nodes: state.nodes.map((node) => (node.id === state.leaderId ? { ...node, partitioned: true, role: 'follower' } : node)) };
}

/**
 * recoverPartition 恢复网络并通过更高任期领导者同步落后日志。
 */
function recoverPartition(state: RaftState): RaftState {
  const leaderId = state.candidateId ?? state.nodes.find((node) => !node.partitioned)?.id ?? state.leaderId;
  const leaderLastTerm = lastEntry(state).term;
  return { ...state, tick: state.tick + 1, lastTransition: 'recover', partitionActive: false, leaderId, nodes: state.nodes.map((node) => ({ ...node, partitioned: false, role: node.id === leaderId ? 'leader' : 'follower', logLength: state.log.length, lastLogTerm: leaderLastTerm, matchIndex: state.log.length, nextIndex: state.log.length + 1, commitIndex: state.commitIndex, appliedIndex: state.commitIndex })) };
}

/**
 * rpcFrom 创建 RPC 广播消息。
 */
function rpcFrom(state: RaftState, from: string, label: string): ViewMessage[] {
  return state.nodes
    .filter((node) => node.id !== from)
    .map((node) =>
      processViewMessage(state.tick, { id: deterministicId('raft-rpc', { from, to: node.id, label, tick: state.tick }), from, to: node.id, at: state.tick, label, status: node.partitioned ? 'dropped' : 'delivered' }, `${label} RPC 正在传播或等待确认。`)
    );
}

/**
 * voteReplies 创建投票回复消息。
 */
function voteReplies(state: RaftState, candidateId: string): ViewMessage[] {
  return state.nodes
    .filter((node) => node.id !== candidateId)
    .map((node) =>
      processViewMessage(state.tick, { id: deterministicId('raft-vote', { from: node.id, candidateId, tick: state.tick }), from: node.id, to: candidateId, at: state.tick, label: node.partitioned ? '投票未送达' : state.votes[node.id] ? '同意投票' : '拒绝投票', status: node.partitioned || !state.votes[node.id] ? 'dropped' : 'delivered' }, '投票回复从跟随者返回候选者。')
    );
}

/**
 * yesVotes 统计同意票数量。
 */
function yesVotes(state: RaftState): number {
  return Object.values(state.votes).filter(Boolean).length;
}

/**
 * replicatedCount 统计拥有完整日志的可达节点数。
 */
export function replicatedCount(state: RaftState): number {
  return state.nodes.filter((node) => node.matchIndex >= state.log.length && !node.partitioned).length;
}

/**
 * quorum 计算 Raft 多数派阈值。
 */
export function quorum(state: RaftState): number {
  return majorityThreshold(state.nodes.length);
}

/**
 * labelRaftNode 把节点 ID 转成用户可读标签。
 */
export function labelRaftNode(state: RaftState, id: string): string {
  return state.nodes.find((node) => node.id === id)?.label ?? id;
}

/**
 * lastEntry 返回领导者日志中最后一条记录。
 */
function lastEntry(state: RaftState) {
  return state.log[state.log.length - 1] ?? { index: 0, term: 0, command: '', committed: false };
}

/**
 * nodeLogUpToDate 实现 RequestVote 的最后日志任期与索引比较规则。
 */
function nodeLogUpToDate(candidate: RaftNode, voterLastIndex: number, voterLastTerm: number): boolean {
  if (candidate.lastLogTerm !== voterLastTerm) return candidate.lastLogTerm > voterLastTerm;
  return candidate.logLength >= voterLastIndex;
}

/**
 * explainRaftPhase 生成阶段说明。
 */
function explainRaftPhase(index: number) {
  const phase = raftPhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
