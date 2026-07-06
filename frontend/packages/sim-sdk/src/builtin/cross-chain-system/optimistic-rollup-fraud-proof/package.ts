// 本文件装配 Optimistic Rollup 欺诈证明仿真包。

import type { SimPackage } from '../../../types';
import { createInitialOptimisticRollupState, optimisticRollupCheckpoint, reduceOptimisticRollupEvent } from './kernel';
import type { OptimisticRollupState } from './model';
import { optimisticRollupCodeTrace, optimisticRollupNarrative } from './trace';
import { renderOptimisticRollupView } from './view';

export const optimisticRollupFraudProofSimulation: SimPackage<OptimisticRollupState> = {
  meta: { code: 'builtin__cross-optimistic-rollup-fraud-proof', name: 'Optimistic Rollup 欺诈证明推演', category: 'cross-chain-system', version: '1.0.0', compute: 'frontend', summary: '完整推演 L2 batch 提交、挑战窗口、交互式二分、L1 单步证明和裁决回滚。', learningObjectives: ['理解 optimistic rollup 为什么需要挑战窗口', '掌握交互式二分如何减少 L1 计算', '区分 batch pending、finalized 和 reverted'], scaleLimit: { nodes: 80, maxTick: 140, maxEvents: 260 } },
  initState: createInitialOptimisticRollupState,
  reducer: reduceOptimisticRollupEvent,
  interactions: [
    { id: 'select', kind: 'select-element', label: '选择 batch 或争议步骤', description: '查看 batch、争议区间和状态根。', emits: 'select', target: 'element', elementFilter: 'rollup-dispute' },
    { id: 'advance', kind: 'button', label: '推进挑战流程', description: '按 optimistic rollup 欺诈证明流程推进。', emits: 'advance', labelTag: 'normal' },
    { id: 'attack', kind: 'button', label: '提交错误状态根', description: '模拟 sequencer 提交错误 claimedRoot。', emits: 'attack', labelTag: 'attack' },
    { id: 'recover', kind: 'button', label: '提交正确状态根', description: '修正 claimedRoot 并进入最终确认。', emits: 'recover', labelTag: 'perturb' },
  ],
  render: renderOptimisticRollupView,
  narrative: optimisticRollupNarrative,
  codeTrace: optimisticRollupCodeTrace,
  checkpoints: [{ id: 'optimistic-rollup-verdict', label: '欺诈证明裁决状态正确', evaluate: (state) => optimisticRollupCheckpoint(state as OptimisticRollupState) }],
};
