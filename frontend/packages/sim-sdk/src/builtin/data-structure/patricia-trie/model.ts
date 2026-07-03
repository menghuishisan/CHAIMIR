// 本文件定义 Patricia Trie 状态树仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface TrieEntry {
  key: string;
  path: string;
  value: string;
  hash: string;
  updated: boolean;
  missing: boolean;
}

export interface PatriciaTrieState extends SimState {
  phaseIndex: number;
  entries: TrieEntry[];
  rootHash: string;
  proofKey: string;
  proofValid: boolean;
  lastTransition: string;
}

export const patriciaTriePhases = [
  { id: 'encode', label: '编码键路径', detail: 'key 转 nibble 路径', effect: '账户或存储 key 被编码成可逐段匹配的路径。', reason: 'Trie 查找依赖路径前缀,不是线性扫描。' },
  { id: 'compress', label: '压缩公共前缀', detail: '合并单子节点路径', effect: '只有一个子分支的路径被压缩成扩展节点。', reason: '路径压缩降低树高和存储成本。' },
  { id: 'insert', label: '插入或更新值', detail: '沿路径改叶子', effect: '更新只影响目标路径及其祖先节点。', reason: '局部更新会通过哈希向根传播。' },
  { id: 'rehash', label: '重算根哈希', detail: '自底向上重算', effect: '被影响路径上的节点重新计算哈希。', reason: '根哈希承诺了整棵状态树。' },
  { id: 'absence', label: '生成缺失证明', detail: '证明路径断点', effect: '对不存在 key 给出路径断点和相邻节点。', reason: '缺失证明说明 key 不在树中,不是节点漏返回。' },
];
