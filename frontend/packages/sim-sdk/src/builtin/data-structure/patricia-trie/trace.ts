// 本文件定义 Patricia Trie 仿真的代码追踪和叙事配置。

import { phaseNarrative } from '../../packageTools';
import { patriciaTriePhases } from './model';

export const patriciaTrieSource = [
  'function updateTrie(key, value) {',
  '  path = encodeNibbles(key);',
  '  node = walkCompressedPath(path);',
  '  leaf.value = value;',
  '  root = rehashToRoot(leaf);',
  '  proveAbsence(missingKey);',
  '}',
];

export const patriciaTrieNarrative = phaseNarrative(patriciaTriePhases, 'trie-root-valid');

export const patriciaTrieCodeTrace = {
  sourceCode: patriciaTrieSource.join('\n'),
  language: 'pseudocode' as const,
  lineMapping: patriciaTriePhases.map((phase, index) => ({ line: Math.min(index + 2, patriciaTrieSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'rehash' ? ('success' as const) : ('normal' as const) })),
  variableWatch: [{ name: 'rootHash', extract: 'state.rootHash', format: 'hex' as const }],
};

/**
 * traceLinesForPatriciaTrie 返回当前 Trie 阶段对应的代码行。
 */
export function traceLinesForPatriciaTrie(transition: string): number[] {
  const index = patriciaTriePhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 2, patriciaTrieSource.length)];
}
