// 本文件把 Merkle Tree 状态转换为树、矩阵和流程三种语义可视化。

import type { MatrixCell, TreeNode, ViewSpec } from '../../../types';
import { fnv1aHex } from '../../../runtime/deterministic';
import { matrixPattern, pipelinePattern, treePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../dataView';
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
  const leftHash = fnv1aHex(`${state.items[0].hash}:${state.items[1].hash}`, 12);
  const rightHash = fnv1aHex(`${state.items[2].hash}:${state.items[3].hash}`, 12);
  return { id: 'mtree-root', label: '根', hash: state.rootHash, children: [{ id: 'mtree-left', label: '左父', hash: leftHash, children: state.items.slice(0, 2).map((item) => ({ id: item.id, label: item.label, hash: item.hash })) }, { id: 'mtree-right', label: '右父', hash: rightHash, children: state.items.slice(2).map((item) => ({ id: item.id, label: item.label, hash: item.hash })) }] };
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
