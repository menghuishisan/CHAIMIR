// 本文件定义 Gas 计量仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface GasStep {
  op: string;
  cost: number;
  executed: boolean;
  failed: boolean;
}

export interface GasState extends SimState {
  phaseIndex: number;
  gasLimit: number;
  gasUsed: number;
  refund: number;
  steps: GasStep[];
  outOfGas: boolean;
  lastTransition: string;
}

export const gasPhases = [
  { id: 'limit', label: '设置 gasLimit', detail: '用户给出上限', effect: '交易声明最多愿意消耗多少 gas。', reason: 'gasLimit 是执行资源的硬边界。' },
  { id: 'meter', label: '逐指令扣费', detail: '执行 op 扣 gas', effect: '运行时每执行一条指令都累计 gasUsed。', reason: '按指令计费让计算和存储成本显式化。' },
  { id: 'oog', label: '处理 gas 不足', detail: '失败并回滚', effect: '当 gasUsed 超过 gasLimit 时执行失败并回滚状态。', reason: 'out-of-gas 防止无限执行耗尽节点资源。' },
  { id: 'refund', label: '计算退款', detail: '释放存储返还', effect: '符合条件的存储释放产生有限退款。', reason: '退款鼓励清理状态,但不能无限抵扣。' },
  { id: 'settle', label: '结算费用', detail: '扣除实际费用', effect: '交易按实际消耗和费用参数结算。', reason: '费用结算决定用户支付和出块者收益。' },
];
