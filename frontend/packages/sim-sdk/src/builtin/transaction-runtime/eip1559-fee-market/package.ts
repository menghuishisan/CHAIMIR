// 本文件装配 EIP-1559 费用市场仿真包。

import type { SimPackage } from '../../../types';
import { createInitialFeeMarketState, feeMarketCheckpoint, reduceFeeMarketEvent } from './kernel';
import type { FeeMarketState } from './model';
import { feeMarketCodeTrace, feeMarketNarrative } from './trace';
import { renderFeeMarketView } from './view';

export const eip1559FeeMarketSimulation: SimPackage<FeeMarketState> = {
  meta: { code: 'builtin__runtime-eip1559-fee-market', name: 'EIP-1559 费用市场推演', category: 'transaction-runtime', version: '1.0.0', compute: 'frontend', summary: '完整推演交易报价、区块选择、base fee 销毁、小费支付和下一块 base fee 调整。', learningObjectives: ['区分 maxFee、priority fee 和实际支付', '理解 base fee 如何随区块负载反馈调整', '观察低报价交易为什么不能入块'], scaleLimit: { nodes: 64, maxTick: 120, maxEvents: 240 } },
  initState: createInitialFeeMarketState,
  reducer: reduceFeeMarketEvent,
  interactions: [
    { id: 'select', kind: 'select-element', label: '选择交易', description: '查看交易报价、入块和费用拆分状态。', emits: 'select', target: 'element', elementFilter: 'eip-tx' },
    { id: 'advance', kind: 'button', label: '推进费用市场', description: '按 EIP-1559 规则推进一个阶段。', emits: 'advance', labelTag: 'normal' },
    { id: 'attack', kind: 'button', label: '制造拥堵', description: '加入高 gas 需求交易,观察 base fee 上升。', emits: 'attack', labelTag: 'attack' },
    { id: 'recover', kind: 'button', label: '提高低价报价', description: '让低价等待交易重新报价。', emits: 'recover', labelTag: 'perturb' },
  ],
  render: renderFeeMarketView,
  narrative: feeMarketNarrative,
  codeTrace: feeMarketCodeTrace,
  checkpoints: [{ id: 'eip1559-fee-split', label: '费用拆分和 base fee 调整正确', evaluate: (state) => feeMarketCheckpoint(state as FeeMarketState) }],
};
