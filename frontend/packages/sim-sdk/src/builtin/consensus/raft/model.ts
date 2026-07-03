// 本文件定义 Raft 选举与日志复制仿真的领域模型,不包含状态迁移和渲染逻辑。

import type { SimState } from '../../../types';
import type { ViewMessage } from '../consensusView';

export type RaftRole = 'follower' | 'candidate' | 'leader';

export interface RaftNode {
  id: string;
  label: string;
  role: RaftRole;
  term: number;
  votedFor?: string;
  logLength: number;
  lastLogTerm: number;
  matchIndex: number;
  nextIndex: number;
  commitIndex: number;
  appliedIndex: number;
  partitioned: boolean;
}

export interface RaftEntry {
  index: number;
  term: number;
  command: string;
  committed: boolean;
}

export interface RaftState extends SimState {
  phaseIndex: number;
  term: number;
  commitIndex: number;
  leaderId: string;
  candidateId?: string;
  nodes: RaftNode[];
  log: RaftEntry[];
  messages: ViewMessage[];
  votes: Record<string, boolean>;
  partitionActive: boolean;
  lastTransition: string;
}

export const raftPhases = [
  { id: 'timeout', label: '选举超时', detail: '跟随者计时器到期', effect: '某个跟随者未收到心跳后进入候选状态并递增任期。', reason: '随机化超时降低多个候选者同时竞选的概率。' },
  { id: 'request-vote', label: '请求投票', detail: '候选者广播 RequestVote', effect: '候选者携带任期和最后日志索引请求其他节点投票。', reason: 'Raft 只允许日志至少同样新的候选者赢得选举。' },
  { id: 'win-election', label: '赢得多数票', detail: '获得过半选票', effect: '候选者获得多数票后成为领导者并开始发送心跳。', reason: '多数派交集保证同一任期最多一个领导者。' },
  { id: 'append-entry', label: '追加日志', detail: '领导者复制命令', effect: '领导者把客户端命令追加到自己的日志并发送 AppendEntries。', reason: '日志复制以领导者为唯一入口,跟随者通过前置索引保持一致。' },
  { id: 'replicate', label: '多数派复制', detail: '更新 matchIndex', effect: '跟随者确认日志后,领导者更新各节点 matchIndex。', reason: '只有复制到多数派的新任期日志才能推进提交索引。' },
  { id: 'commit', label: '提交日志', detail: '推进 commitIndex', effect: '领导者把多数派已复制的日志标记为已提交并通过心跳告知跟随者。', reason: '提交索引定义状态机可以安全执行到的位置。' },
  { id: 'recover', label: '分区恢复', detail: '修复落后日志', effect: '网络恢复后,新领导者用更高任期和日志覆盖落后节点。', reason: '任期和日志匹配规则让分区后的集群重新收敛。' },
] as const;
