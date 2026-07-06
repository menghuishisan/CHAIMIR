// 本文件定义 Tendermint/CometBFT 轮次、锁定和提交仿真的状态模型。

import type { SimState } from '../../../types';
import type { ViewMessage } from '../consensusView';

export interface TendermintValidator {
  id: string;
  label: string;
  power: number;
  online: boolean;
  lockedValue?: string;
  prevote?: string;
  precommit?: string;
}

export interface TendermintProposal {
  id: string;
  proposer: string;
  value: string;
  round: number;
  valid: boolean;
}

export interface TendermintRoundsState extends SimState {
  phaseIndex: number;
  height: number;
  round: number;
  proposal?: TendermintProposal;
  validators: TendermintValidator[];
  messages: ViewMessage[];
  committedValue?: string;
  timeout: boolean;
  lastTransition: string;
}

export const tendermintRoundPhases = [
  { id: 'proposal', label: 'Proposal', detail: '提议者广播区块值', effect: '当前 round 的 proposer 广播候选区块值。', reason: 'Tendermint 每轮先有一个提议值,后续投票围绕该值展开。' },
  { id: 'prevote', label: 'Prevote', detail: '验证者预投票', effect: '验证者检查提议并发送 prevote。', reason: 'prevote 阶段用于形成是否值得锁定该值的初步共识。' },
  { id: 'precommit', label: 'Precommit', detail: '超过 2/3 后预提交', effect: '超过三分之二投票后,验证者锁定值并发送 precommit。', reason: '锁定规则防止下一轮随意改投造成安全性破坏。' },
  { id: 'commit', label: 'Commit', detail: '超过 2/3 precommit', effect: '超过三分之二 precommit 后,区块值被提交。', reason: '提交阈值保证至少一个诚实验证者集合交叉。' },
  { id: 'timeout', label: 'Timeout/New Round', detail: '超时进入下一轮', effect: '未达到阈值时进入下一 round 并携带 valid value。', reason: '活性依赖超时推进,安全性依赖锁定约束。' },
  { id: 'lock', label: 'Lock 约束', detail: '锁定值限制改投', effect: '已锁定验证者只能在满足解锁条件时改投。', reason: 'lock 是 Tendermint 安全性的核心状态。' },
] as const;
