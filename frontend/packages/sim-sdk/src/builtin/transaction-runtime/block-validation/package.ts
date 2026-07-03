// 本文件装配区块验证与拒绝仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { blockAccepted, createInitialBlockValidationState, reduceBlockValidationEvent } from './kernel';
import { blockValidationCodeTrace, blockValidationNarrative } from './trace';
import { renderBlockValidationView } from './view';
import type { BlockValidationState } from './model';

export const blockValidationSimulation: SimPackage<BlockValidationState> = {
  meta: { code: 'builtin__runtime-block-validation', name: '区块验证与拒绝推演', category: 'transaction-runtime', version: '1.0.0', compute: 'frontend', summary: '完整推演区块头、交易根、收据根、状态根校验和无效区块拒绝。', learningObjectives: ['理解节点为什么要本地验证区块', '掌握各类根摘要的职责', '观察无效区块如何被拒绝'], scaleLimit: { nodes: 48, maxTick: 120, maxEvents: 220 } },
  initState: createInitialBlockValidationState,
  reducer: reduceBlockValidationEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择校验项', description: '查看区块验证项。', emits: 'select', target: 'element', elementFilter: 'validation-item' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进区块验证流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '篡改状态根', description: '模拟出块者给出错误状态根。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '重算根摘要', description: '本地执行后重算所有根摘要。', emits: 'recover', labelTag: 'perturb' }],
  render: renderBlockValidationView,
  narrative: blockValidationNarrative,
  codeTrace: blockValidationCodeTrace,
  checkpoints: [{ id: 'block-validation-accepted', label: '区块验证通过', evaluate: (state) => blockAccepted(state as BlockValidationState) }],
};
