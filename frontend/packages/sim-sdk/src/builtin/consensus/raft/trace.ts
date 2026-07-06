// 本文件定义 Raft 仿真的代码追踪和教学叙事。

import type { CodeTraceDef } from '../../../types';
import { phaseNarrative } from '../../packageTools';
import { raftPhases } from './model';

export const raftSource = [
  'function raft(command) {',
  '  candidate = onElectionDelay();',
  '  votes = requestVote(candidate.term, candidate.lastLogIndex, candidate.lastLogTerm);',
  '  require(votes.majority() && candidate.logAtLeastAsNew());',
  '  leader.append(command);',
  '  appendEntries(prevLogIndex, prevLogTerm, entries);',
  '  if (prevLogMatches() && acknowledgements.majority()) commitIndex++;',
  '  heartbeat(commitIndex);',
  '  repairLaggingFollowers(nextIndex--, appendEntries);',
  '}',
];

/**
 * traceLinesForRaft 把 Raft 内核迁移映射到对应伪代码行。
 */
export function traceLinesForRaft(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    timeout: [2],
    'request-vote': [2, 3],
    'win-election': [3, 4],
    'append-entry': [5],
    replicate: [6, 7],
    commit: [7, 8],
    recover: [9],
  };
  return mapping[transition] ?? [1];
}

export const raftCodeTrace: CodeTraceDef = {
  sourceCode: raftSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == timeout || lastTransition == request-vote', annotation: '跟随者超时后递增任期并成为候选者。' },
    { line: 3, triggerCondition: 'lastTransition == request-vote', annotation: 'RequestVote 携带最后日志索引和任期。' },
    { line: 4, triggerCondition: 'lastTransition == win-election', annotation: '只有多数票且日志不落后的候选者才能成为领导者。', highlightStyle: 'success' },
    { line: 5, triggerCondition: 'lastTransition == append-entry', annotation: '领导者是日志追加的唯一入口。' },
    { line: 6, triggerCondition: 'lastTransition == replicate', annotation: 'AppendEntries 使用 prevLogIndex 和 prevLogTerm 做一致性检查。' },
    { line: 7, triggerCondition: 'lastTransition == replicate || lastTransition == commit', annotation: '多数派确认后推进 commitIndex。', highlightStyle: 'success' },
    { line: 9, triggerCondition: 'lastTransition == recover', annotation: '网络恢复后通过 nextIndex 回退修复落后日志。' },
  ],
  variableWatch: [
    { name: 'term', extract: 'state.term', format: 'number' },
    { name: 'commitIndex', extract: 'state.commitIndex', format: 'number' },
    { name: 'leaderId', extract: 'state.leaderId', format: 'string' },
    { name: 'lastLogIndex', extract: 'state.log.length', format: 'number' },
    { name: 'appliedNodes', extract: 'state.nodes.filter(node => node.appliedIndex >= state.commitIndex).length', format: 'number' },
  ],
};

export const raftNarrative = phaseNarrative(raftPhases, 'raft-majority-commit');
