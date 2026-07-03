// 本文件把 Patricia Trie 状态转换为树、矩阵和流程三种语义可视化。

import type { MatrixCell, TreeNode, ViewSpec } from '../../../types';
import { fnv1aHex } from '../../../runtime/deterministic';
import { matrixPattern, pipelinePattern, treePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../dataView';
import { patriciaTriePhases, type PatriciaTrieState } from './model';

/**
 * renderPatriciaTrieView 基于内核状态生成 Patricia Trie 可视化。
 */
export function renderPatriciaTrieView(state: PatriciaTrieState): ViewSpec {
  return { summary: `根哈希 ${state.rootHash.slice(0, 8)},证明 key ${state.proofKey},缺失证明${state.proofValid ? '通过' : '等待'}。`, patterns: [treePattern('trie-tree', 'Patricia Trie', trieRoot(state), highlightedPath(state), 'main'), matrixPattern('trie-matrix', '路径校验', state.entries.map((entry) => entry.key), ['路径', '值', '哈希'], trieCells(state), 'side'), pipelinePattern('trie-pipeline', 'Trie 更新流程', pipelineSteps(patriciaTriePhases, state.phaseIndex, !state.proofValid && state.phaseIndex >= 4), patriciaTriePhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * trieRoot 构造压缩路径树。
 */
function trieRoot(state: PatriciaTrieState): TreeNode {
  return { id: 'trie-root', label: 'root', hash: state.rootHash, children: [{ id: 'trie-a', label: 'al*', hash: fnv1aHex('al', 8), children: state.entries.filter((entry) => entry.key.startsWith('ali')).map((entry) => ({ id: entry.key, label: entry.key, hash: entry.hash })) }, { id: 'trie-b', label: 'bo*', hash: fnv1aHex('bo', 8), children: state.entries.filter((entry) => entry.key.startsWith('bo')).map((entry) => ({ id: entry.key, label: entry.key, hash: entry.hash })) }] };
}

/**
 * highlightedPath 返回缺失证明路径。
 */
function highlightedPath(state: PatriciaTrieState): string[] {
  return state.proofKey.startsWith('al') ? ['trie-root', 'trie-a'] : ['trie-root'];
}

/**
 * trieCells 展示路径、值和哈希。
 */
function trieCells(state: PatriciaTrieState): MatrixCell[][] {
  return matrixCells(state.entries.map((entry) => entry.key), ['路径', '值', '哈希'], (row, column) => {
    const entry = state.entries.find((item) => item.key === row);
    if (!entry) return { label: '无', status: 'empty' };
    if (column === '路径') return { label: entry.path, status: 'yes' };
    if (column === '值') return { label: entry.value, status: entry.updated ? 'pending' : 'yes' };
    return { label: entry.hash.slice(0, 6), status: state.proofValid ? 'yes' : 'fault' };
  });
}
