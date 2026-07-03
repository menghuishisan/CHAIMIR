// 本文件定义 Merkle Tree 结构仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface MerkleItem {
  id: string;
  label: string;
  value: string;
  hash: string;
  updated: boolean;
}

export interface MerkleTreeState extends SimState {
  phaseIndex: number;
  items: MerkleItem[];
  rootHash: string;
  proofPath: string[];
  dirtyLeafId?: string;
  lastTransition: string;
}

export const merkleTreePhases = [
  { id: 'sort', label: '稳定排序叶子', detail: '按 key 排序', effect: '叶子按稳定顺序排列。', reason: '排序不稳定会导致同一集合得到不同根。' },
  { id: 'leaf', label: '计算叶子哈希', detail: '数据摘要化', effect: '每个数据项转换为叶子摘要。', reason: '叶子摘要是树上所有父节点哈希的输入。' },
  { id: 'pair', label: '成对合并', detail: '左右节点组合', effect: '相邻叶子按左右顺序合并为父节点。', reason: '左右顺序是 Merkle 根可复现的关键。' },
  { id: 'root', label: '生成根摘要', detail: '递归到根', effect: '父节点继续合并直到得到唯一根。', reason: '根摘要承诺整棵树的全部叶子。' },
  { id: 'update', label: '局部更新路径', detail: '只重算受影响路径', effect: '单个叶子变更只重算该叶子到根的路径。', reason: '局部重算让 Merkle Tree 支持高效更新。' },
];
