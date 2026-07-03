// 本文件装配跨链最终性确认仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialFinalityState, finalitySafe, reduceFinalityEvent } from './kernel';
import { finalityCodeTrace, finalityNarrative } from './trace';
import { renderFinalityView } from './view';
import type { FinalityState } from './model';

export const finalityConfirmationSimulation: SimPackage<FinalityState> = {
  meta: { code: 'builtin__cross-finality-confirmation', name: '跨链最终性确认推演', category: 'cross-chain-system', version: '1.0.0', compute: 'frontend', summary: '完整推演跨链源链确认数、最终性证明、重组风险检测、等待策略和确认后释放。', learningObjectives: ['理解为什么跨链需要等待最终性', '掌握确认数和重组风险关系', '观察最终性闸门如何保护目标链执行'], scaleLimit: { nodes: 48, maxTick: 120, maxEvents: 220 } },
  initState: createInitialFinalityState,
  reducer: reduceFinalityEvent,
  interactions: [{ id: 'advance', kind: 'button', label: '推进确认', description: '增加源链确认数并检查最终性。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '模拟源链重组', description: '让事件所在区块被回滚。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '等待最终性', description: '等待足够确认并提交最终性证明。', emits: 'recover', labelTag: 'perturb' }],
  render: renderFinalityView,
  narrative: finalityNarrative,
  codeTrace: finalityCodeTrace,
  checkpoints: [{ id: 'finality-release-safe', label: '最终性确认后释放', evaluate: (state) => finalitySafe(state as FinalityState) }],
};
