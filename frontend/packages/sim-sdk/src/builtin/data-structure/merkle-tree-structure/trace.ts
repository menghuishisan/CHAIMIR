// 本文件定义 Merkle Tree 结构仿真的代码追踪和叙事配置。

import { phaseNarrative } from '../../packageTools';
import { merkleTreePhases } from './model';

export const merkleTreeSource = [
  'function buildMerkleTree(items) {',
  '  leaves = sort(items).map(hash);',
  '  while leaves.length > 1:',
  '    leaves = pairwiseHash(leaves);',
  '  return leaves[0];',
  '  updatePath(changedLeaf);',
  '}',
];

export const merkleTreeNarrative = phaseNarrative(merkleTreePhases, 'merkle-tree-root-valid');

export const merkleTreeCodeTrace = {
  sourceCode: merkleTreeSource.join('\n'),
  language: 'pseudocode' as const,
  lineMapping: merkleTreePhases.map((phase, index) => ({ line: Math.min(index + 2, merkleTreeSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'root' ? ('success' as const) : ('normal' as const) })),
  variableWatch: [{ name: 'rootHash', extract: 'state.rootHash', format: 'hex' as const }],
};

/**
 * traceLinesForMerkleTree 返回当前 Merkle Tree 阶段对应的代码行。
 */
export function traceLinesForMerkleTree(transition: string): number[] {
  const index = merkleTreePhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 2, merkleTreeSource.length)];
}
