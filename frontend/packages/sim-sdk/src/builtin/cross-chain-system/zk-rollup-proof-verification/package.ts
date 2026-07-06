// 本文件装配 ZK Rollup 批次证明与验证仿真包。

import type { SimPackage } from '../../../types';
import { createInitialZkRollupState, reduceZkRollupEvent, zkRollupCheckpoint } from './kernel';
import type { ZkRollupState } from './model';
import { zkRollupCodeTrace, zkRollupNarrative } from './trace';
import { renderZkRollupView } from './view';

export const zkRollupProofVerificationSimulation: SimPackage<ZkRollupState> = {
  meta: { code: 'builtin__cross-zk-rollup-proof-verification', name: 'ZK Rollup 批次证明与验证推演', category: 'cross-chain-system', version: '1.0.0', compute: 'frontend', summary: '完整推演 L2 batch 聚合、witness/trace、validity proof、L1 verifier 和状态根更新/拒绝。', learningObjectives: ['理解 validity proof 如何绑定 public inputs', '区分 proof 生成和 L1 验证', '观察错误 public input 为什么不能更新状态根'], scaleLimit: { nodes: 80, maxTick: 140, maxEvents: 260 } },
  initState: createInitialZkRollupState,
  reducer: reduceZkRollupEvent,
  interactions: [
    { id: 'select', kind: 'select-element', label: '选择证明输入', description: '查看 proof、public inputs 和状态根绑定。', emits: 'select', target: 'element', elementFilter: 'zk-rollup-input' },
    { id: 'advance', kind: 'button', label: '推进证明流程', description: '按 ZK Rollup 有效性证明流程推进。', emits: 'advance', labelTag: 'normal' },
    { id: 'attack', kind: 'button', label: '篡改公开输入', description: '让 public input 与 newRoot 不匹配。', emits: 'attack', labelTag: 'attack' },
    { id: 'recover', kind: 'button', label: '修正证明输入', description: '恢复 public input 与 proof 绑定。', emits: 'recover', labelTag: 'perturb' },
  ],
  render: renderZkRollupView,
  narrative: zkRollupNarrative,
  codeTrace: zkRollupCodeTrace,
  checkpoints: [{ id: 'zk-rollup-verifier', label: 'ZK Rollup proof 验证结果正确', evaluate: (state) => zkRollupCheckpoint(state as ZkRollupState) }],
};
