// 本文件把 Merkle Tree 状态转换为树、矩阵和流程三种语义可视化。

import type { MatrixCell, TreeNode, TeachingFrame } from '../../../types';
import { teachingFrame, matrixPattern, pipelinePattern, treePattern, selectedOrFrameFocus } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../dataView';
import { merkleStructureParentHash } from '../dataPrimitives';
import { merkleTreePhases, type MerkleTreeState } from './model';

/**
 * renderMerkleTreeView 基于内核状态生成 Merkle Tree 可视化。
 */
export function renderMerkleTreeView(state: MerkleTreeState): TeachingFrame {
  const dirtyLeaf = state.items.find((item) => item.id === state.dirtyLeafId);
    const summary = `根摘要 ${state.rootHash.slice(0, 8)},叶子 ${state.items.length} 个,更新叶子 ${dirtyLeaf?.label ?? '无'},重算路径 ${state.proofPath.length} 层。`;
  const patterns = [treePattern('merkle-structure-tree', 'Merkle Tree 自底向上重算路径', treeRoot(state), state.proofPath), matrixPattern('merkle-structure-matrix', '叶子哈希与脏写传播矩阵', state.items.map((item) => item.label), ['值', '叶子哈希', '是否触发重算'], merkleCells(state)), pipelinePattern('merkle-structure-pipeline', '叶子更新 -> 父节点重算 -> 根摘要更新流程', pipelineSteps(merkleTreePhases, state.phaseIndex, Boolean(state.dirtyLeafId)), merkleTreePhases[state.phaseIndex].id)];
  return teachingFrame({
    summary,
    phase: {
      id: state.phase,
      title: state.explanation.title,
      intent: 'observe',
      what: state.explanation.effect,
      why: state.explanation.reason,
      watch: summary,
    },
    focus: {
      primary: selectedOrFrameFocus(state.selectedElementId, ['merkle-structure-tree']),
      secondary: ['merkle-structure-matrix', 'merkle-structure-pipeline'],
    },
    layout: {
      primary: 'merkle-structure-tree',
      evidence: ['merkle-structure-matrix', 'merkle-structure-pipeline'],
    },
    patterns,
  });
}

/**
 * treeRoot 构造树形数据。
 */
function treeRoot(state: MerkleTreeState): TreeNode {
  let level = state.items.map<TreeNode>((item) => ({ id: item.id, label: item.label, hash: item.hash }));
  let depth = 0;
  while (level.length > 1) {
    const padded = level.length % 2 === 0 ? level : level.concat({ ...level[level.length - 1], id: `${level[level.length - 1].id}-dup-l${depth}`, label: `${level[level.length - 1].label} 复制` });
    const next: TreeNode[] = [];
    for (let index = 0; index < padded.length; index += 2) {
      const nextLength = Math.ceil(padded.length / 2);
      next.push({
        id: nextLength === 1 ? 'mtree-root' : `mtree-root-level-${depth + 1}-${index / 2}`,
        label: nextLength === 1 ? '根' : `第 ${depth + 1} 层节点 ${index / 2 + 1}`,
        hash: merkleStructureParentHash(padded[index].hash, padded[index + 1].hash),
        children: [padded[index], padded[index + 1]],
      });
    }
    level = next;
    depth += 1;
  }
  return level[0] ?? { id: 'mtree-root', label: '根', hash: state.rootHash };
}

/**
 * merkleCells 展示叶子值、哈希和更新状态。
 */
function merkleCells(state: MerkleTreeState): MatrixCell[][] {
  return matrixCells(state.items.map((item) => item.label), ['值', '哈希', '更新'], (row, column) => {
    const item = state.items.find((entry) => entry.label === row);
    if (!item) return { label: '无', status: 'empty' };
    if (column === '值') return { label: item.value, status: item.updated ? 'pending' : 'yes' };
    if (column === '叶子哈希') return { label: item.hash.slice(0, 6), status: 'yes' };
    return { label: item.updated ? '已改' : '未改', status: item.updated ? 'pending' : 'empty' };
  });
}
