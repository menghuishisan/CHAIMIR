// 本文件定义 Nonce 顺序仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface NonceTx {
  id: string;
  nonce: number;
  fee: number;
  status: 'pending' | 'blocked' | 'included' | 'replaced';
}

export interface NonceState extends SimState {
  phaseIndex: number;
  accountNonce: number;
  txs: NonceTx[];
  gapDetected: boolean;
  lastTransition: string;
}

export const noncePhases = [
  { id: 'read', label: '读取账户 nonce', detail: '获取下一序号', effect: '钱包读取账户当前下一笔交易序号。', reason: 'Nonce 是账户交易顺序和防重放的核心字段。' },
  { id: 'submit', label: '提交连续交易', detail: '按序进入交易池', effect: '多个交易带着连续 nonce 进入交易池。', reason: '同一账户交易必须按 nonce 顺序执行。' },
  { id: 'gap', label: '识别 nonce 缺口', detail: '缺少前序交易', effect: '后序交易因为前序 nonce 缺失而阻塞。', reason: '缺口不补齐,后续交易不能越序执行。' },
  { id: 'replace', label: '替换卡住交易', detail: '提高手续费', effect: '同 nonce 更高手续费交易替换旧交易。', reason: '替换交易是解决低费卡住的标准方式。' },
  { id: 'include', label: '按序打包执行', detail: '依次推进 nonce', effect: '区块按 nonce 顺序包含交易并推进账户 nonce。', reason: '执行后账户 nonce 与链上历史保持一致。' },
];
