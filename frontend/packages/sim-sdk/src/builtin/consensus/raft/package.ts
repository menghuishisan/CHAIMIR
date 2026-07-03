// 本文件把 Raft 内核、视图、叙事和检查点装配为 SimPackage 入口。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { createInitialRaftState, raftMajorityCommit, raftSingleLeader, reduceRaftEvent } from './kernel';
import type { RaftState } from './model';
import { raftCodeTrace, raftNarrative } from './trace';
import { renderRaftView } from './view';

/**
 * raftSimulation 将 Raft 选举与日志复制暴露给 M4 运行时。
 */
export const raftSimulation: SimPackage<RaftState> = {
  meta: {
    code: 'builtin__raft-log-replication',
    name: 'Raft 选举与日志复制推演',
    category: 'consensus',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演 Raft 选举超时、RequestVote、多数派领导、AppendEntries、提交索引和分区恢复。',
    learningObjectives: ['理解任期与多数派选举', '掌握日志匹配和提交索引推进', '观察网络分区后如何恢复一致日志'],
    scaleLimit: { nodes: 96, maxTick: 160, maxEvents: 280 },
  },
  initState: createInitialRaftState,
  reducer: reduceRaftEvent,
  interactions: commonAlgorithmInteractions('raft-node'),
  render: renderRaftView,
  narrative: raftNarrative,
  codeTrace: raftCodeTrace,
  checkpoints: [
    { id: 'raft-majority-commit', label: '多数派提交成立', evaluate: (state) => raftMajorityCommit(state as RaftState) },
    { id: 'raft-single-leader', label: '单任期单领导者', evaluate: (state) => raftSingleLeader(state as RaftState) },
  ],
};

