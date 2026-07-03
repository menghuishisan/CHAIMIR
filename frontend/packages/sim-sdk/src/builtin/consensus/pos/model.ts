// 本文件定义 PoS 权益证明与最终性仿真的领域模型。

import type { SimState } from '../../../types';
import type { ViewMessage } from '../consensusView';

export interface PosValidator {
  id: string;
  label: string;
  stake: number;
  proposer: boolean;
  attested: boolean;
  slashed: boolean;
  online: boolean;
}

export interface PosAttestation {
  validatorId: string;
  blockRoot: string;
  sourceEpoch: number;
  targetEpoch: number;
  signature: string;
  valid: boolean;
}

export interface PosSlashing {
  validatorId: string;
  reason: 'double-vote' | 'surround-vote';
  evidenceRoots: string[];
}

export interface PosState extends SimState {
  phaseIndex: number;
  slot: number;
  epoch: number;
  randomness: string;
  blockRoot: string;
  committee: string[];
  aggregateSignature?: string;
  conflictingRoot?: string;
  validators: PosValidator[];
  attestations: PosAttestation[];
  slashings: PosSlashing[];
  messages: ViewMessage[];
  justifiedEpoch: number;
  finalizedEpoch: number;
  samples: Array<{ x: number; quorum: number; risk: number; finality: number }>;
  lastTransition: string;
}

export const posPhases = [
  { id: 'randomness', label: '生成随机种子', detail: '混合历史随机性', effect: '协议用历史随机性和 slot 信息生成可验证种子。', reason: '随机选主降低持续垄断出块的概率。' },
  { id: 'proposer', label: '选择区块提议者', detail: '按权益权重抽取', effect: '系统根据权益权重选出当前 slot 的提议者。', reason: 'PoS 的出块概率与质押权益相关,但仍需要随机性防操纵。' },
  { id: 'propose', label: '提议新区块', detail: '广播区块根', effect: '提议者构造区块并向验证者广播区块根。', reason: '验证者只对满足规则的区块根进行见证。' },
  { id: 'attest', label: '验证者见证', detail: '签名投票', effect: '在线验证者检查区块并签名见证目标检查点。', reason: '见证票按权益加权,多数权益支持后才能推进最终性。' },
  { id: 'justify', label: '证明检查点', detail: '达到三分之二权益', effect: '当前 epoch 的检查点获得超过三分之二权益见证后被证明。', reason: 'Casper FFG 用权益多数证明检查点链。' },
  { id: 'finalize', label: '最终确定检查点', detail: '连续证明', effect: '连续两个检查点被证明后,较早检查点最终确定。', reason: '最终性让已确认历史不能被无罚没地重写。' },
  { id: 'slash', label: '处理双签罚没', detail: '识别冲突见证', effect: '检测同一验证者对冲突目标双签后执行罚没。', reason: '罚没把破坏安全性的行为变成经济损失。' },
] as const;
