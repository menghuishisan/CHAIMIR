// 本文件定义闪电贷组合攻击仿真的状态模型和阶段表。

import type { SimState } from '../../../types';
import type { SecurityActor, SecurityCall } from '../securityView';

export interface FlashLoanState extends SimState {
  phaseIndex: number;
  baseLoanAmount: number;
  basePoolPrice: number;
  loanAmount: number;
  poolPrice: number;
  protocolDebt: number;
  attackerProfit: number;
  limitEnabled: boolean;
  containedAttempt: boolean;
  actors: SecurityActor[];
  calls: SecurityCall[];
  lastTransition: string;
}

export const flashLoanPhases = [
  { id: 'borrow', label: '瞬时借入资金', detail: '同交易借款', effect: '攻击者在一笔交易内借入大量流动性。', reason: '闪电贷无需抵押,但必须在交易结束前归还。' },
  { id: 'manipulate', label: '操纵市场状态', detail: '推偏价格或储备', effect: '大额资金短暂改变池子价格或协议状态。', reason: '许多协议假设单交易内状态不会被剧烈改变。' },
  { id: 'exploit', label: '调用目标协议', detail: '按异常状态获利', effect: '目标协议按被操纵状态执行借款或兑换。', reason: '漏洞来自协议读取了可被同交易操纵的状态。' },
  { id: 'repay', label: '归还闪电贷', detail: '交易末尾还款', effect: '攻击者归还本金和费用后留下利润。', reason: '闪电贷攻击的所有步骤必须在原子交易中完成。' },
  { id: 'limit', label: '限额与延迟防护', detail: '限制单块影响', effect: '协议启用限额、冷却时间和价格保护。', reason: '限制单交易影响面能切断闪电贷组合攻击。' },
];
