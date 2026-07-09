// 本文件定义 ZK Rollup 证明验证仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { zkRollupPhases } from './model';

export const zkRollupSource = [
  'function submitValidityBatch(batch) {',
  '  trace = execute(batch.transactions);',
  '  publicInputs = [oldRoot, newRoot];',
  '  proof = prover.generate(trace, publicInputs);',
  '  require(verifier.verify(proof, publicInputs));',
  '  rollupStateRoot = newRoot;',
  '}',
];

export const zkRollupNarrative = phaseNarrative(zkRollupPhases, 'zk-rollup-verifier');

export const zkRollupCodeTrace = {
  sourceCode: zkRollupSource.join('\n'),
  language: 'pseudocode' as const,
  lineMapping: zkRollupPhases.map((phase, index) => ({ line: Math.min(index + 1, zkRollupSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'update' ? ('success' as const) : phase.id === 'reject' ? ('error' as const) : ('normal' as const) })),
  variableWatch: [
    { name: 'proofValid', extract: 'state.proofValid', format: 'bool' as const },
    { name: 'verifierAccepted', extract: 'state.verifierAccepted', format: 'bool' as const },
    { name: 'newRoot', extract: 'state.newRoot', format: 'string' as const },
  ],
};

/** traceLinesForZkRollup 根据当前阶段返回零知识汇总验证伪代码高亮行。 */
export function traceLinesForZkRollup(transition: string): number[] {
  const index = zkRollupPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, zkRollupSource.length)];
}
