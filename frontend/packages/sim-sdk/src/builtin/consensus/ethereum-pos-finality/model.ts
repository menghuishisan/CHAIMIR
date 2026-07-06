// 本文件定义 Ethereum PoS LMD-GHOST 与 Casper FFG 仿真的状态模型。

import type { SimState } from '../../../types';

export interface EthPosValidator {
  id: string;
  label: string;
  weight: number;
  online: boolean;
  latestVote?: string;
}

export interface EthPosBlock {
  id: string;
  slot: number;
  epoch: number;
  parent: string;
  weight: number;
  status: 'genesis' | 'candidate' | 'head' | 'justified' | 'finalized' | 'orphaned';
}

export interface EthPosAttestation {
  id: string;
  validatorId: string;
  blockId: string;
  epoch: number;
  delivered: boolean;
}

export interface EthPosFinalityState extends SimState {
  phaseIndex: number;
  slot: number;
  epoch: number;
  head: string;
  justified: string;
  finalized: string;
  validators: EthPosValidator[];
  blocks: EthPosBlock[];
  attestations: EthPosAttestation[];
  participationHistory: Array<{ x: number; quorum: number; risk: number; finality: number }>;
  lastTransition: string;
}

export const ethPosFinalityPhases = [
  { id: 'propose', label: 'Slot 提议区块', detail: 'proposer 扩展当前 head', effect: '提议者在当前 slot 生成新区块并连接到本地 head。', reason: 'PoS 每个 slot 只有一个主要提议者,但网络延迟可能产生分叉。' },
  { id: 'attest', label: '验证者投最新消息', detail: 'attestation 指向区块', effect: '在线验证者发布对目标区块的最新投票。', reason: 'LMD-GHOST 只使用每个验证者的最新消息计算 head。' },
  { id: 'ghost', label: '执行 LMD-GHOST', detail: '按权重选择 head', effect: '协议从 justified checkpoint 开始,沿权重最高子树选择链头。', reason: '链头选择和最终性是两层规则,不能混为一谈。' },
  { id: 'justify', label: '证明 checkpoint', detail: '达到三分之二权益', effect: '目标 epoch 获得超过三分之二权益投票后被 justified。', reason: 'Casper FFG 用 checkpoint 投票推进经济最终性。' },
  { id: 'finalize', label: '最终确定 checkpoint', detail: '连续证明', effect: '连续 checkpoint 被证明后,较早 checkpoint 被 finalized。', reason: 'finalized checkpoint 之后的历史不能无罚没地重写。' },
  { id: 'delay', label: '处理延迟投票', detail: 'head 可变但 finality 稳定', effect: '延迟投票可能改变 head 权重,但不会自动回滚 finalized checkpoint。', reason: '学生需要区分短期 fork choice 与长期最终性。' },
] as const;
