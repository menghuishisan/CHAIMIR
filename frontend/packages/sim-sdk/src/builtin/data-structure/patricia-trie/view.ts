// 本文件把 Patricia Trie 状态转换为树、矩阵和流程三种语义可视化。

import type { MatrixCell, TreeNode, TeachingFrame } from '../../../types';
import { fnv1aHex } from '../../../runtime/deterministic';
import { teachingFrame, matrixPattern, pipelinePattern, treePattern, selectedOrFrameFocus } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../dataView';
import { patriciaTriePhases, type PatriciaTrieState } from './model';

/**
 * renderPatriciaTrieView 基于内核状态生成 Patricia Trie 可视化。
 */
export function renderPatriciaTrieView(state: PatriciaTrieState): TeachingFrame {
  const path = highlightedPath(state);
    const summary = `根哈希 ${state.rootHash.slice(0, 8)},证明 key ${state.proofKey},压缩路径 ${path.join(' -> ')},缺失证明${state.proofValid ? '通过' : '等待'}。`;
  const patterns = [treePattern('trie-tree', 'Patricia Trie 压缩路径证明', trieRoot(state), path), matrixPattern('trie-matrix', 'Key Nibble 路径与节点哈希矩阵', state.entries.map((entry) => entry.key), ['压缩路径', '值', '节点哈希'], trieCells(state)), pipelinePattern('trie-pipeline', 'Key 拆分 -> 路径查找 -> 根哈希验证流程', pipelineSteps(patriciaTriePhases, state.phaseIndex, !state.proofValid && state.phaseIndex >= 4), patriciaTriePhases[state.phaseIndex].id)];
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
      primary: selectedOrFrameFocus(state.selectedElementId, ['trie-tree']),
      secondary: ['trie-matrix', 'trie-pipeline'],
    },
    layout: {
      primary: 'trie-tree',
      evidence: ['trie-matrix', 'trie-pipeline'],
    },
    patterns,
  });
}

/**
 * trieRoot 构造压缩路径树。
 */
function trieRoot(state: PatriciaTrieState): TreeNode {
  const groups = prefixGroups(state.entries.map((entry) => entry.key));
  return {
    id: 'trie-root',
    label: 'root',
    hash: state.rootHash,
    children: groups.map((prefix) => ({ id: `trie-${prefix}`, label: `${prefix}*`, hash: fnv1aHex(prefix, 8), children: state.entries.filter((entry) => entry.key.startsWith(prefix)).map((entry) => ({ id: entry.key, label: entry.key, hash: entry.hash })) })),
  };
}

/**
 * highlightedPath 返回缺失证明路径。
 */
function highlightedPath(state: PatriciaTrieState): string[] {
  const prefix = prefixGroups(state.entries.map((entry) => entry.key)).find((item) => state.proofKey.startsWith(item));
  return prefix ? ['trie-root', `trie-${prefix}`] : ['trie-root'];
}

/**
 * prefixGroups 根据键集合计算压缩路径的首段分组。
 */
function prefixGroups(keys: string[]): string[] {
  return Array.from(new Set(keys.map((key) => key.slice(0, Math.min(2, key.length)) || 'key'))).sort();
}

/**
 * trieCells 展示路径、值和哈希。
 */
function trieCells(state: PatriciaTrieState): MatrixCell[][] {
  return matrixCells(state.entries.map((entry) => entry.key), ['路径', '值', '哈希'], (row, column) => {
    const entry = state.entries.find((item) => item.key === row);
    if (!entry) return { label: '无', status: 'empty' };
    if (column === '压缩路径') return { label: entry.path, status: 'yes' };
    if (column === '值') return { label: entry.value, status: entry.updated ? 'pending' : 'yes' };
    return { label: entry.hash.slice(0, 6), status: state.proofValid ? 'yes' : 'fault' };
  });
}
