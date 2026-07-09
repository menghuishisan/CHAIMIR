// 本文件定义 EIP-1559 费用市场仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { feeMarketPhases } from './model';

export const feeMarketSource = [
  'function buildBlock(mempool, baseFee) {',
  '  candidates = filter(tx.maxFeePerGas >= baseFee);',
  '  selected = sortByEffectiveTip(candidates);',
  '  gasUsed = execute(selected);',
  '  burn(baseFee * gasUsed);',
  '  nextBaseFee = adjust(baseFee, gasUsed, targetGas);',
  '}',
];

export const feeMarketNarrative = phaseNarrative(feeMarketPhases, 'eip1559-fee-split');

export const feeMarketCodeTrace = {
  sourceCode: feeMarketSource.join('\n'),
  language: 'pseudocode' as const,
  lineMapping: feeMarketPhases.map((phase, index) => ({ line: Math.min(index + 1, feeMarketSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'adjust' ? ('success' as const) : ('normal' as const) })),
  variableWatch: [
    { name: 'baseFee', extract: 'state.baseFee', format: 'number' as const },
    { name: 'gasUsed', extract: 'state.gasUsed', format: 'number' as const },
    { name: 'nextBaseFee', extract: 'state.nextBaseFee', format: 'number' as const },
  ],
};

/** traceLinesForFeeMarket 根据当前阶段返回 EIP-1559 伪代码高亮行。 */
export function traceLinesForFeeMarket(transition: string): number[] {
  const index = feeMarketPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, feeMarketSource.length)];
}
