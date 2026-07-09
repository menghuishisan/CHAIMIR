// 本文件定义 mempool 替换交易仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { mempoolReplacementPhases } from './model';

export const mempoolReplacementSource = [
  'function acceptTx(tx) {',
  '  expected = accountNonce(tx.sender);',
  '  if tx.nonce > expected: queue(tx);',
  '  if sameNonceExists(tx): require(priceBump(tx));',
  '  gossipLocalPool(tx);',
  '  includeContinuousNonce();',
  '}',
];

export const mempoolReplacementNarrative = phaseNarrative(mempoolReplacementPhases, 'mempool-replacement-valid');

export const mempoolReplacementCodeTrace = {
  sourceCode: mempoolReplacementSource.join('\n'),
  language: 'pseudocode' as const,
  lineMapping: mempoolReplacementPhases.map((phase, index) => ({ line: Math.min(index + 1, mempoolReplacementSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'replace' ? ('error' as const) : ('normal' as const) })),
  variableWatch: [
    { name: 'expectedNonce', extract: 'state.expectedNonce.Alice', format: 'number' as const },
    { name: 'replacementRequiredBump', extract: 'state.replacementRequiredBump', format: 'number' as const },
  ],
};

/** traceLinesForMempoolReplacement 根据当前阶段返回伪代码高亮行。 */
export function traceLinesForMempoolReplacement(transition: string): number[] {
  const index = mempoolReplacementPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, mempoolReplacementSource.length)];
}
