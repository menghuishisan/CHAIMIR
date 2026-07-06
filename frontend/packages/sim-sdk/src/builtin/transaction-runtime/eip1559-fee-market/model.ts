// 本文件定义 EIP-1559 费用市场仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface FeeMarketTx {
  id: string;
  sender: string;
  gasLimit: number;
  maxFeePerGas: number;
  maxPriorityFeePerGas: number;
  included: boolean;
  dropped: boolean;
  paid: number;
  burned: number;
  tip: number;
  refunded: number;
}

export interface FeeMarketPoint {
  x: number;
  baseFee: number;
  gasUsed: number;
  tip: number;
}

export interface FeeMarketState extends SimState {
  phaseIndex: number;
  blockNumber: number;
  baseFee: number;
  targetGas: number;
  gasUsed: number;
  nextBaseFee: number;
  congested: boolean;
  transactions: FeeMarketTx[];
  history: FeeMarketPoint[];
  lastTransition: string;
}

export const feeMarketPhases = [
  { id: 'quote', label: '交易报价', detail: '声明 maxFee 与小费', effect: '用户交易带着最高总价和最高小费进入交易池。', reason: 'EIP-1559 把协议销毁的 base fee 和给出块者的小费拆开。' },
  { id: 'select', label: '构建区块', detail: '按有效小费选择交易', effect: '构建器只选择 maxFee 覆盖当前 base fee 的交易,再按有效小费排序。', reason: '低于 base fee 的交易不能进入当前区块。' },
  { id: 'execute', label: '执行计量', detail: '累计 gasUsed', effect: '区块执行被选交易并累计 gasUsed。', reason: 'gasUsed 和 targetGas 的差值决定下一块 base fee 的方向。' },
  { id: 'split', label: '费用拆分', detail: '销毁 base fee 并支付小费', effect: '每笔入块交易按实际 gas 把 base fee 销毁,把 priority fee 支付给验证者。', reason: '销毁机制让拥堵成本回到协议层,小费只表达排序激励。' },
  { id: 'adjust', label: '调整 base fee', detail: '比较 targetGas', effect: '如果 gasUsed 高于目标值,下一块 base fee 上升;低于目标值则下降。', reason: 'base fee 是按区块容量反馈自动调节的价格信号。' },
  { id: 'settle', label: '进入下一块', detail: '继承新 base fee', effect: '下一块使用更新后的 base fee,未入块交易继续等待或重报价。', reason: '连续区块的价格反馈让学生看到拥堵和费用的动态关系。' },
] as const;
