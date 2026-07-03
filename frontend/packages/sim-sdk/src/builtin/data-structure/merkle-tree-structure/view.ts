// 本文件把 Merkle Tree 状态转换为树、矩阵和流程三种语义可视化。

import type { MatrixCell, TreeNode, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern, treePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../dataView';
import { merkleStructureParentHash } from '../dataPrimitives';
import { merkleTreePhases, type MerkleTreeState } from './model';

/**
 * renderMerkleTreeView 基于内核状态生成 Merkle Tree 可视化。
 */
export function renderMerkleTreeView(state: MerkleTreeState): ViewSpec {
  return { summary: `根摘要 ${state.rootHash.slice(0, 8)},更新路径 ${state.proofPath.length} 层。`, patterns: [treePattern('merkle-structure-tree', 'Merkle Tree 结构', treeRoot(state), state.proofPath, 'main'), matrixPattern('merkle-structure-matrix', '叶子状态', state.items.map((item) => item.label), ['值', '哈希', '更新'], merkleCells(state), 'side'), pipelinePattern('merkle-structure-pipeline', '树构建流程', pipelineSteps(merkleTreePhases, state.phaseIndex, Boolean(state.dirtyLeafId)), merkleTreePhases[state.phaseIndex].id, 'bottom')] };
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
    if (column === '哈希') return { label: item.hash.slice(0, 6), status: 'yes' };
    return { label: item.updated ? '已改' : '未改', status: item.updated ? 'pending' : 'empty' };
  });
}
