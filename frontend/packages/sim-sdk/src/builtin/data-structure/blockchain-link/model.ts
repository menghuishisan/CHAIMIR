// 本文件定义区块链父哈希结构仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface BlockLink {
  id: string;
  height: number;
  payload: string;
  hash: string;
  parentHash: string;
  canonical: boolean;
  forked: boolean;
}

export interface BlockchainLinkState extends SimState {
  phaseIndex: number;
  blocks: BlockLink[];
  fork: BlockLink[];
  reorganized: boolean;
  lastTransition: string;
}

export const blockchainPhases = [
  { id: 'genesis', label: '创建创世块', detail: '初始化链根', effect: '链从没有父块的创世块开始。', reason: '创世块是所有后续父哈希校验的根。' },
  { id: 'append', label: '追加新区块', detail: '写入父哈希', effect: '新区块记录前一区块哈希并形成线性链接。', reason: '父哈希把区块顺序变成可校验的依赖链。' },
  { id: 'validate', label: '校验链接', detail: '逐高度比较', effect: '验证器逐块检查 parentHash 是否等于前一块 hash。', reason: '任何插入或篡改都会破坏后续链接。' },
  { id: 'fork', label: '识别分叉', detail: '同高度多块', effect: '当同一父块出现多个子块时标记分叉。', reason: '分叉是分布式出块下的正常现象,必须显式表示。' },
  { id: 'reorg', label: '规范链重组', detail: '选择更优分支', effect: '当分叉链更优时切换规范链并孤立旧块。', reason: '重组让所有节点最终收敛到同一条规范历史。' },
];
