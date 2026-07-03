// 本文件把 Merkle 证明内核状态映射为封闭可视化模式。

import type { MatrixCell, TreeNode, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern, treePattern } from '../../packageTools';
import { binaryTree, matrixCells, pipelineSteps } from '../cryptoView';
import { merkleParentHash } from '../cryptoPrimitives';
import { labelMerkleLeaf, rootHash } from './kernel';
import { merkleProofPhases, type MerkleProofState } from './model';

/**
 * renderMerkleProofView 输出证明树、证明材料矩阵和验证流程。
 */
export function renderMerkleProofView(state: MerkleProofState): ViewSpec {
  return {
    summary: `目标叶子 ${labelMerkleLeaf(state, state.targetLeafId)},证明路径 ${state.proofPath.length} 层,校验${state.proofValid ? '通过' : '未通过'}。`,
    patterns: [
      treePattern('merkle-tree', 'Merkle 证明树', merkleRoot(state), state.proofPath, 'main'),
      matrixPattern('merkle-matrix', '证明材料', proofRows(state), ['摘要', '状态'], proofCells(state), 'side'),
      pipelinePattern('merkle-pipeline', '证明验证流程', pipelineSteps([...merkleProofPhases], state.phaseIndex, !state.proofValid && state.phaseIndex >= 3), merkleProofPhases[state.phaseIndex].id, 'bottom'),
    ],
  };
}

/**
 * merkleRoot 构造树形可视化数据。
 */
function merkleRoot(state: MerkleProofState): TreeNode {
  return binaryTree(
    'merkle-root',
    state.leaves,
    (left, right) => merkleParentHash(left, right)
  );
}

/**
 * proofCells 展示证明材料是否有效。
 */
function proofCells(state: MerkleProofState): MatrixCell[][] {
  return matrixCells(proofRows(state), ['摘要', '状态'], (row, column) => {
    if (column === '摘要') return { label: row === '根' ? rootHash(state.leaves).slice(0, 6) : '已提供', status: 'yes' };
    return { label: state.proofValid ? '通过' : row === '根' ? '不匹配' : '受影响', status: state.proofValid ? 'yes' : row === '根' ? 'fault' : 'pending' };
  });
}

/**
 * proofRows 输出目标叶子和证明兄弟摘要的展示行。
 */
function proofRows(state: MerkleProofState): string[] {
  return ['目标叶子'].concat(state.proofSiblings.map((sibling, index) => `兄弟 ${index + 1}:${sibling}`), '根');
}
