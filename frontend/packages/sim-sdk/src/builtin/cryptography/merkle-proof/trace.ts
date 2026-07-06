// 本文件定义 Merkle 证明仿真的代码追踪和教学叙事。

import type { CodeTraceDef } from '../../../types';
import { phaseNarrative } from '../../packageTools';
import { merkleProofPhases } from './model';

export const merkleProofSource = [
  'function verifyMerkle(leaf, siblings, expectedRoot) {',
  '  hash = H(leaf);',
  '  for sibling in siblings:',
  '    hash = H(ordered(hash, sibling));',
  '  require(hash == expectedRoot);',
  '  return true;',
  '}',
];

/**
 * traceLinesForMerkleProof 把 Merkle 证明内核迁移映射到伪代码行。
 */
export function traceLinesForMerkleProof(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    'leaf-hash': [2],
    path: [3],
    combine: [3, 4],
    compare: [5, 6],
    locate: [4, 5],
    tamper: [2, 5],
  };
  return mapping[transition] ?? [1];
}

export const merkleProofCodeTrace: CodeTraceDef = {
  sourceCode: merkleProofSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == leaf-hash || lastTransition == tamper', annotation: '目标叶子先被压缩为固定摘要。' },
    { line: 3, triggerCondition: 'lastTransition == path || lastTransition == combine', annotation: '证明只遍历目标叶子的兄弟路径。' },
    { line: 4, triggerCondition: 'lastTransition == combine || lastTransition == locate', annotation: '每一层必须按左右顺序合并哈希。' },
    { line: 5, triggerCondition: 'lastTransition == compare || lastTransition == locate || lastTransition == tamper', annotation: '重建根必须等于可信根。', highlightStyle: 'success' },
    { line: 6, triggerCondition: 'lastTransition == compare', annotation: '根一致才接受证明。', highlightStyle: 'success' },
  ],
  variableWatch: [
    { name: 'computedRoot', extract: 'state.computedRoot', format: 'hex' },
    { name: 'expectedRoot', extract: 'state.expectedRoot', format: 'hex' },
    { name: 'proofValid', extract: 'state.proofValid', format: 'bool' },
  ],
};

export const merkleProofNarrative = phaseNarrative(merkleProofPhases, 'merkle-proof-valid');
