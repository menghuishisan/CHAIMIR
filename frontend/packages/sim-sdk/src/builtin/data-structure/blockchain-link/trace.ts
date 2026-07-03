// 本文件定义区块链父哈希结构仿真的伪代码追踪和叙事配置。

import { phaseNarrative } from '../../packageTools';
import { blockchainPhases } from './model';

export const blockchainSource = [
  'function validateChain(blocks) {',
  '  require(blocks[0].parentHash == "genesis");',
  '  for i in 1..blocks.length:',
  '    require(blocks[i].parentHash == blocks[i-1].hash);',
  '  markForksByHeight();',
  '  chooseCanonicalBranch();',
  '}',
];

export const blockchainNarrative = phaseNarrative(blockchainPhases, 'blockchain-link-valid');

export const blockchainCodeTrace = {
  sourceCode: blockchainSource.join('\n'),
  language: 'pseudocode' as const,
  lineMapping: blockchainPhases.map((phase, index) => ({ line: Math.min(index + 2, blockchainSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'reorg' ? ('success' as const) : ('normal' as const) })),
  variableWatch: [{ name: 'height', extract: 'state.metrics.height', format: 'number' as const }],
};

/**
 * traceLinesForBlockchainLink 返回当前阶段需要高亮的伪代码行。
 */
export function traceLinesForBlockchainLink(transition: string): number[] {
  const index = blockchainPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 2, blockchainSource.length)];
}
