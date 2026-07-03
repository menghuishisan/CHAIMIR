// 本文件定义 PoW 最长链仿真的领域模型,不包含状态迁移和渲染逻辑。

import type { SimState } from '../../../types';
import type { ViewMessage } from '../consensusView';

export interface PowMiner {
  id: string;
  label: string;
  hashPower: number;
  validTip: string;
  accepted: boolean;
  attacker: boolean;
}

export interface PowAttempt {
  nonce: number;
  hash: string;
  score: number;
  valid: boolean;
}

export interface PowBlock {
  id: string;
  height: number;
  minerId: string;
  parentHash: string;
  hash: string;
  nonce: number;
  work: number;
  canonical: boolean;
  attacker: boolean;
}

export interface PowState extends SimState {
  phaseIndex: number;
  difficulty: number;
  targetPrefix: string;
  mempoolSize: number;
  candidateNonce: number;
  candidateHash: string;
  candidateParentHash: string;
  candidateMinerId: string;
  hashAttempts: PowAttempt[];
  targetSpacing: number;
  miners: PowMiner[];
  blocks: PowBlock[];
  privateFork: PowBlock[];
  messages: ViewMessage[];
  samples: Array<{ x: number; quorum: number; risk: number; finality: number }>;
  selfishMining: boolean;
  lastTransition: string;
}

export const powPhases = [
  { id: 'mempool', label: '交易进入内存池', detail: '收集待打包交易', effect: '节点接收交易并按费用与依赖关系放入内存池。', reason: 'PoW 的竞争对象是包含交易和父区块哈希的候选区块。' },
  { id: 'assemble', label: '构造候选区块', detail: '选择父块和交易', effect: '矿工选择当前累计工作量最高的链尖作为父区块。', reason: '父块选择决定后续最长链竞争的基准。' },
  { id: 'hash-search', label: '执行哈希搜索', detail: '枚举 nonce', effect: '矿工不断改变 nonce,直到区块哈希低于难度目标。', reason: '哈希搜索把出块概率绑定到算力,让篡改历史需要重做工作量。' },
  { id: 'broadcast', label: '广播新区块', detail: '传播候选块', effect: '找到有效 nonce 的矿工向全网广播新区块。', reason: '广播延迟会造成临时分叉,但节点仍按有效工作量选择链。' },
  { id: 'validate', label: '验证工作量', detail: '校验父块和目标', effect: '节点校验父哈希、交易和哈希目标,无效块不会扩展本地链。', reason: 'PoW 安全性依赖每个节点独立验证,而不是信任矿工声明。' },
  { id: 'longest-chain', label: '选择累计工作量最高链', detail: '处理临时分叉', effect: '节点比较累计工作量,把最高工作量链作为规范链。', reason: '最长链规则让网络在传播延迟后重新收敛。' },
  { id: 'adjust', label: '调整难度', detail: '按出块速度调目标', effect: '系统根据窗口内出块速度调整目标难度。', reason: '难度调整让平均出块时间在算力变化下保持稳定。' },
] as const;
