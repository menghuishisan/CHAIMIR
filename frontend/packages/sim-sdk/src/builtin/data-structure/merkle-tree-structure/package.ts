// 本文件装配 Merkle Tree 结构仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialMerkleTreeState, merkleTreeRootValid, reduceMerkleTreeEvent } from './kernel';
import { merkleTreeCodeTrace, merkleTreeNarrative } from './trace';
import { renderMerkleTreeView } from './view';
import type { MerkleTreeState } from './model';

export const merkleTreeStructureSimulation: SimPackage<MerkleTreeState> = {
  meta: { code: 'builtin__data-merkle-tree-structure', name: 'Merkle Tree 构建更新推演', category: 'data-structure', version: '1.0.0', compute: 'frontend', summary: '完整推演 Merkle Tree 的叶子排序、叶子哈希、成对合并、根摘要生成和局部更新路径。', learningObjectives: ['理解 Merkle Tree 根如何生成', '掌握左右顺序和稳定排序的重要性', '观察局部更新为何只影响一条路径'], scaleLimit: { nodes: 80, maxTick: 120, maxEvents: 220 } },
  initState: createInitialMerkleTreeState,
  reducer: reduceMerkleTreeEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择叶子', description: '选择叶子查看更新路径。', emits: 'select', target: 'element', elementFilter: 'merkle-leaf' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进 Merkle Tree 构建流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '修改叶子', description: '修改一个叶子观察根变化。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '重建根', description: '沿受影响路径重算根摘要。', emits: 'recover', labelTag: 'perturb' }],
  render: renderMerkleTreeView,
  narrative: merkleTreeNarrative,
  codeTrace: merkleTreeCodeTrace,
  checkpoints: [{ id: 'merkle-tree-root-valid', label: 'Merkle Tree 根有效', evaluate: (state) => merkleTreeRootValid(state as MerkleTreeState) }],
};
