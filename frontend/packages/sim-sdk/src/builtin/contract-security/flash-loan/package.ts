// 本文件装配闪电贷组合攻击仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialFlashLoanState, flashContained, reduceFlashLoanEvent } from './kernel';
import { flashLoanCodeTrace, flashLoanNarrative } from './trace';
import { renderFlashLoanView } from './view';
import type { FlashLoanState } from './model';

export const flashLoanSimulation: SimPackage<FlashLoanState> = {
  meta: { code: 'builtin__security-flash-loan', name: '闪电贷组合攻击推演', category: 'contract-security', version: '1.0.0', compute: 'frontend', summary: '完整推演闪电贷借款、市场状态操纵、目标协议调用、同交易还款、利润结算和限额防护。', learningObjectives: ['理解闪电贷原子性', '掌握组合攻击步骤', '观察限额和延迟如何降低单交易冲击'], scaleLimit: { nodes: 64, maxTick: 120, maxEvents: 220 } },
  initState: createInitialFlashLoanState,
  reducer: reduceFlashLoanEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择协议', description: '查看组合调用中的协议状态。', emits: 'select', target: 'element', elementFilter: 'security-actor' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进闪电贷攻击流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '执行组合攻击', description: '完成借款、操纵和获利。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '启用限额防护', description: '限制单交易影响并恢复价格保护。', emits: 'recover', labelTag: 'perturb' }],
  render: renderFlashLoanView,
  narrative: flashLoanNarrative,
  codeTrace: flashLoanCodeTrace,
  checkpoints: [{ id: 'flash-loan-contained', label: '闪电贷冲击受控', evaluate: (state) => flashContained(state as FlashLoanState) }],
};
