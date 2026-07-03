// 本文件装配 Gas 计量与回滚仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialGasState, gasSettled, reduceGasEvent } from './kernel';
import { gasCodeTrace, gasNarrative } from './trace';
import { renderGasView } from './view';
import type { GasState } from './model';

export const gasMeteringSimulation: SimPackage<GasState> = {
  meta: { code: 'builtin__runtime-gas-metering', name: 'Gas 计量与回滚推演', category: 'transaction-runtime', version: '1.0.0', compute: 'frontend', summary: '完整推演 gasLimit、逐指令扣费、out-of-gas 回滚、退款上限和最终费用结算。', learningObjectives: ['理解 gasLimit 是资源硬边界', '掌握逐指令扣费和回滚', '观察退款为什么需要上限'], scaleLimit: { nodes: 48, maxTick: 100, maxEvents: 200 } },
  initState: createInitialGasState,
  reducer: reduceGasEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择指令', description: '查看指令 gas 消耗。', emits: 'select', target: 'element', elementFilter: 'gas-op' }, { id: 'advance', kind: 'button', label: '执行一步', description: '执行下一条指令并扣 gas。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '降低 gasLimit', description: '模拟 gasLimit 不足。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '提高 gasLimit', description: '提高上限后重新执行。', emits: 'recover', labelTag: 'perturb' }],
  render: renderGasView,
  narrative: gasNarrative,
  codeTrace: gasCodeTrace,
  checkpoints: [{ id: 'gas-execution-settled', label: 'Gas 已正确结算', evaluate: (state) => gasSettled(state as GasState) }],
};
