// 本文件装配交易生命周期仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialTxLifecycleState, receiptReady, reduceTxLifecycleEvent } from './kernel';
import { txLifecycleCodeTrace, txLifecycleNarrative } from './trace';
import { renderTxLifecycleView } from './view';
import type { TxLifecycleState } from './model';

export const transactionLifecycleSimulation: SimPackage<TxLifecycleState> = {
  meta: { code: 'builtin__runtime-transaction-lifecycle', name: '交易生命周期推演', category: 'transaction-runtime', version: '1.0.0', compute: 'frontend', summary: '完整推演交易从构造、签名、交易池、区块打包、执行到回执确认的生命周期。', learningObjectives: ['理解交易提交到确认的每个阶段', '掌握交易池和区块打包的区别', '观察失败交易如何产生回执'], scaleLimit: { nodes: 64, maxTick: 120, maxEvents: 220 } },
  initState: createInitialTxLifecycleState,
  reducer: reduceTxLifecycleEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择参与方', description: '查看交易生命周期状态。', emits: 'select', target: 'element', elementFilter: 'runtime-actor' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进交易生命周期。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '模拟交易丢弃', description: '模拟交易因费用不足被交易池丢弃。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '重新提交', description: '提高费用并重新提交交易。', emits: 'recover', labelTag: 'perturb' }],
  render: renderTxLifecycleView,
  narrative: txLifecycleNarrative,
  codeTrace: txLifecycleCodeTrace,
  checkpoints: [{ id: 'tx-lifecycle-receipt', label: '交易回执已生成', evaluate: (state) => receiptReady(state as TxLifecycleState) }],
};
