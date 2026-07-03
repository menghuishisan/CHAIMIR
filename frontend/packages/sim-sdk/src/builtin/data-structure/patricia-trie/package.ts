// 本文件装配 Patricia Trie 状态树仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialPatriciaTrieState, patriciaTrieValid, reducePatriciaTrieEvent } from './kernel';
import { patriciaTrieCodeTrace, patriciaTrieNarrative } from './trace';
import { renderPatriciaTrieView } from './view';
import type { PatriciaTrieState } from './model';

export const patriciaTrieSimulation: SimPackage<PatriciaTrieState> = {
  meta: { code: 'builtin__data-patricia-trie', name: 'Patricia Trie 状态树推演', category: 'data-structure', version: '1.0.0', compute: 'frontend', summary: '完整推演 Patricia Trie 的键路径编码、公共前缀压缩、插入更新、根哈希重算和缺失证明。', learningObjectives: ['理解状态树如何按 key 路径组织', '掌握路径压缩和根哈希传播', '观察缺失证明如何成立'], scaleLimit: { nodes: 96, maxTick: 140, maxEvents: 240 } },
  initState: createInitialPatriciaTrieState,
  reducer: reducePatriciaTrieEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择路径', description: '选择 Trie 路径节点查看哈希传播。', emits: 'select', target: 'element', elementFilter: 'trie-node' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进 Trie 更新流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '写入错误叶子', description: '模拟叶子值与根哈希不一致。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '重算根哈希', description: '从叶子向根重算哈希。', emits: 'recover', labelTag: 'perturb' }],
  render: renderPatriciaTrieView,
  narrative: patriciaTrieNarrative,
  codeTrace: patriciaTrieCodeTrace,
  checkpoints: [{ id: 'trie-root-valid', label: 'Trie 根哈希与证明有效', evaluate: (state) => patriciaTrieValid(state as PatriciaTrieState) }],
};
