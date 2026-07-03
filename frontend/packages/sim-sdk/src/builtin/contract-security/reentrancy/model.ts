// 本文件定义重入攻击仿真的状态模型和阶段表。

import type { SimState } from '../../../types';
import type { SecurityActor, SecurityCall } from '../securityView';

export interface ReentrancyState extends SimState {
  phaseIndex: number;
  vaultBalance: number;
  attackerCredit: number;
  attackerBalance: number;
  lockEnabled: boolean;
  reentered: boolean;
  actors: SecurityActor[];
  calls: SecurityCall[];
  lastTransition: string;
}

export const reentrancyPhases = [
  { id: 'deposit', label: '建立存款余额', detail: '记录用户余额', effect: '金库记录攻击合约的可提现余额。', reason: '重入攻击必须先拥有一次合法提款入口。' },
  { id: 'withdraw', label: '发起提款', detail: '调用 withdraw', effect: '攻击合约调用提款函数。', reason: '漏洞点通常出现在提款流程中。' },
  { id: 'external-call', label: '先外部转账', detail: '发送 ETH', effect: '金库先向攻击合约转账,余额尚未扣减。', reason: '外部调用早于状态更新会把控制权交给攻击者。' },
  { id: 'callback', label: '回调重入', detail: 'fallback 再次提款', effect: '攻击合约在 fallback 中再次调用提款。', reason: '状态未更新时重复进入会绕过余额检查。' },
  { id: 'guard', label: '重入锁修复', detail: '先改状态再转账', effect: '启用锁并按检查-效果-交互顺序执行。', reason: '重入锁和先更新状态能阻断递归提款。' },
];
