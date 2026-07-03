// 本文件定义 HotStuff 仿真的代码追踪和教学叙事。

import type { CodeTraceDef, NarrativeStep } from '../../../types';
import { hotstuffPhases } from './model';

export const hotstuffSource = [
  'function hotstuff(view) {',
  '  highQC = collectNewView();',
  '  proposal = leader.propose(extend(highQC.block));',
  '  votes = replicas.voteIfSafe(proposal, lockedBlock, highQC);',
  '  qc = aggregate(votes, 2 * f + 1);',
  '  lock(qc.block);',
  '  if (threeChain(qc.block.parent.parent, qc.block.parent, qc.block)) commit(grandparent(qc.block));',
  '  onTimeout() moveToNextView(highQC);',
  '}',
];

/**
 * traceLinesForHotStuff 把 HotStuff 内核迁移映射到伪代码高亮行。
 */
export function traceLinesForHotStuff(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    'new-view': [2],
    proposal: [3],
    vote: [4],
    qc: [5, 6],
    'chain-commit': [7],
    pacemaker: [8],
  };
  return mapping[transition] ?? [1];
}

export const hotstuffCodeTrace: CodeTraceDef = {
  sourceCode: hotstuffSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == new-view', annotation: '领导者收集各副本最高 QC,确定安全扩展点。' },
    { line: 3, triggerCondition: 'lastTransition == proposal', annotation: '新提案必须扩展 highQC 所在分支。' },
    { line: 4, triggerCondition: 'lastTransition == vote', annotation: '副本按锁规则判断能否为提案签名。' },
    { line: 5, triggerCondition: 'lastTransition == qc', annotation: '2f+1 投票被聚合成 QC。', highlightStyle: 'success' },
    { line: 6, triggerCondition: 'lastTransition == qc', annotation: '形成 QC 后副本锁定该块,下一视图沿它扩展。' },
    { line: 7, triggerCondition: 'lastTransition == chain-commit', annotation: '连续三代 QC 成立时提交祖父块。', highlightStyle: 'success' },
    { line: 8, triggerCondition: 'lastTransition == pacemaker', annotation: '超时后 pacemaker 推进视图并继承 highQC。', highlightStyle: 'error' },
  ],
  variableWatch: [
    { name: 'view', extract: 'state.view', format: 'number' },
    { name: 'highQcBlock', extract: 'state.highQcBlock', format: 'string' },
    { name: 'lockedBlock', extract: 'state.lockedBlock', format: 'string' },
    { name: 'committedBlock', extract: 'state.committedBlock', format: 'string' },
  ],
};

export const hotstuffNarrative: NarrativeStep[] = hotstuffPhases.map((phase, index) => ({
  id: phase.id,
  title: phase.label,
  trigger: (state) => state.phase === phase.label,
  highlight: [phase.id],
  explain: `${phase.effect} ${phase.reason}`,
  defaultDurationMs: 1200,
  question:
    index === hotstuffPhases.length - 2
      ? {
          prompt: 'HotStuff 三链提交是否依赖连续 QC?',
          options: ['依赖', '不依赖'],
          answer: '依赖',
          checkpointId: 'hotstuff-three-chain',
        }
      : undefined,
}));
