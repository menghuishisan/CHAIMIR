// 本文件定义 PBFT 内置仿真包的伪代码追踪和叙事内容。

import type { CodeTraceDef, NarrativeStep } from '../../../types';
import { pbftPhases } from './model';

export const pbftSourceCode = [
  'function runPbft(request) {',
  '  primary.receive(request)',
  '  digest = hash(view, sequence, request)',
  '  primary.broadcast(PRE_PREPARE, digest)',
  '  replica.rejectIfConflicting(view, sequence, digest)',
  '  replica.broadcast(PREPARE, digest)',
  '  require(countPrepare(digest) >= quorum(n, f))',
  '  replica.broadcast(COMMIT, digest)',
  '  require(countCommit(digest) >= quorum(n, f))',
  '  replica.execute(request)',
  '  client.acceptWhenRepliesAtLeast(f + 1)',
  '  replica.broadcast(CHECKPOINT, stateDigest)',
  '  require(countCheckpoint(stateDigest) >= quorum(n, f))',
  '  if primary.doubleProposes then markFault(primary)',
  '  replica.freezeConflictingPrePrepare()',
  '  replica.sendViewChange(preparedCertificate)',
  '  newPrimary.collect(quorum(n, f), VIEW_CHANGE)',
  '  newPrimary.broadcast(NEW_VIEW, safeDigest)',
  '  resumeFromSafeDigest()',
  '}',
];

export const pbftCodeTrace: CodeTraceDef = {
  sourceCode: pbftSourceCode.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == client-request', annotation: '客户端请求进入主节点日志。' },
    { line: 4, triggerCondition: 'lastTransition == pre-prepare', annotation: '主节点广播 view、sequence、digest。' },
    { line: 5, triggerCondition: 'lastTransition == fault-injected', annotation: '副本拒绝同视图同序号的冲突摘要。', highlightStyle: 'error' },
    { line: 7, triggerCondition: 'lastTransition == prepare-certificate', annotation: '准备证书需要达到 BFT 法定人数的匹配准备票。' },
    { line: 9, triggerCondition: 'lastTransition == commit-certificate', annotation: '提交证书需要达到 BFT 法定人数的匹配提交票。' },
    { line: 11, triggerCondition: 'lastTransition == execute-reply', annotation: '客户端用 f+1 一致回复确认结果。', highlightStyle: 'success' },
    { line: 13, triggerCondition: 'lastTransition == stable-checkpoint', annotation: '稳定检查点用于日志截断和安全继承。', highlightStyle: 'success' },
    { line: 17, triggerCondition: 'lastTransition == new-view', annotation: '新主节点收集达到法定人数的视图切换证书后恢复安全摘要。' },
  ],
  variableWatch: [
    { name: 'view', extract: 'state.view', format: 'number' },
    { name: 'sequence', extract: 'state.sequence', format: 'number' },
    { name: 'quorum', extract: 'state.metrics.quorum', format: 'number' },
    { name: 'preparedVotes', extract: 'state.metrics.preparedVotes', format: 'number' },
    { name: 'commitVotes', extract: 'state.metrics.commitVotes', format: 'number' },
  ],
};

export const pbftNarrative: NarrativeStep[] = pbftPhases.map((phase, index) => ({
  id: phase.id,
  title: phase.label,
  trigger: (state) => state.phase === phase.label,
  highlight: [phase.id],
  explain: `${phase.effect} ${phase.reason}`,
  defaultDurationMs: 1200,
  question:
    index === pbftPhases.length - 1
      ? {
          prompt: 'PBFT 在本轮提交后是否已经满足安全执行条件?',
          options: ['满足', '不满足'],
          answer: '满足',
          checkpointId: 'pbft-safety',
        }
      : undefined,
}));
