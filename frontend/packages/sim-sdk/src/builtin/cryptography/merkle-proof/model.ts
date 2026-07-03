// 本文件定义 Merkle 证明仿真的领域模型和阶段元数据。

import type { SimState } from '../../../types';
import type { MerkleProofStep } from '../cryptoPrimitives';

export interface MerkleLeaf {
  id: string;
  label: string;
  canonicalValue: string;
  value: string;
  hash: string;
  inPath: boolean;
  tampered: boolean;
}

export interface MerkleProofState extends SimState {
  phaseIndex: number;
  leaves: MerkleLeaf[];
  targetLeafId: string;
  proofPath: string[];
  proofSiblings: string[];
  proofSteps: MerkleProofStep[];
  computedRoot: string;
  expectedRoot: string;
  proofValid: boolean;
  lastTransition: string;
}

export const merkleProofPhases = [
  { id: 'leaf-hash', label: '计算叶子哈希', detail: '数据转叶子摘要', effect: '每条交易先被压缩成叶子哈希。', reason: '证明不会暴露整棵树,但必须固定每个叶子的摘要。' },
  { id: 'path', label: '选择兄弟路径', detail: '收集相邻分支', effect: '证明只携带目标叶子到根所需的兄弟哈希。', reason: 'Merkle 证明大小随树高增长,不随总叶子数线性增长。' },
  { id: 'combine', label: '逐层合并', detail: '按左右顺序哈希', effect: '验证者用目标叶子和兄弟路径逐层重建根。', reason: '左右顺序错误会得到完全不同的根。' },
  { id: 'compare', label: '比较根摘要', detail: '对比链上根', effect: '重建根与可信根一致则证明通过。', reason: '可信根承诺了整棵树的内容。' },
  { id: 'locate', label: '定位篡改叶子', detail: '标记失败路径', effect: '当叶子被改动时,失败会沿证明路径向根传播。', reason: '路径高亮能说明是哪条数据破坏了承诺关系。' },
] as const;
