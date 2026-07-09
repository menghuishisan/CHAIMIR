// 本文件定义 Optimistic Rollup 欺诈证明仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { optimisticRollupPhases } from './model';

export const optimisticRollupSource = [
  'function submitOptimisticBatch(batch) {',
  '  postStateRoot(batch.claimedRoot);',
  '  if challenger.detectsMismatch(): openChallenge();',
  '  segment = bisectExecutionTrace(batch);',
  '  fraud = verifyOneStep(segment);',
  '  if fraud: revertBatchAndSlash();',
  '}',
];

export const optimisticRollupNarrative = phaseNarrative(optimisticRollupPhases, 'optimistic-rollup-verdict');

export const optimisticRollupCodeTrace = {
  sourceCode: optimisticRollupSource.join('\n'),
  language: 'pseudocode' as const,
  lineMapping: optimisticRollupPhases.map((phase, index) => ({ line: Math.min(index + 1, optimisticRollupSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'verdict' ? ('success' as const) : phase.id === 'challenge' ? ('error' as const) : ('normal' as const) })),
  variableWatch: [
    { name: 'claimedRoot', extract: 'state.claimedRoot', format: 'string' as const },
    { name: 'expectedRoot', extract: 'state.expectedRoot', format: 'string' as const },
    { name: 'fraudProven', extract: 'state.fraudProven', format: 'bool' as const },
  ],
};

/** traceLinesForOptimisticRollup 根据当前阶段返回欺诈证明伪代码高亮行。 */
export function traceLinesForOptimisticRollup(transition: string): number[] {
  const index = optimisticRollupPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, optimisticRollupSource.length)];
}
