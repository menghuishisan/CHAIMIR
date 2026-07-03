// 本文件装配 Nonce 顺序与替换交易仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialNonceState, nonceValid, reduceNonceEvent } from './kernel';
import { nonceCodeTrace, nonceNarrative } from './trace';
import { renderNonceView } from './view';
import type { NonceState } from './model';

export const nonceOrderingSimulation: SimPackage<NonceState> = {
  meta: { code: 'builtin__runtime-nonce-ordering', name: 'Nonce 顺序与替换交易推演', category: 'transaction-runtime', version: '1.0.0', compute: 'frontend', summary: '完整推演账户 nonce 读取、交易池排序、nonce 缺口阻塞、同 nonce 高费替换和顺序打包执行。', learningObjectives: ['理解 nonce 如何防重放和定序', '掌握 nonce 缺口导致的阻塞', '观察替换交易如何解除卡顿'], scaleLimit: { nodes: 48, maxTick: 100, maxEvents: 200 } },
  initState: createInitialNonceState,
  reducer: reduceNonceEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择交易', description: '查看交易 nonce 状态。', emits: 'select', target: 'element', elementFilter: 'tx' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进 nonce 排序流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '制造 nonce 缺口', description: '移除前序交易造成阻塞。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '替换交易', description: '用更高手续费补齐缺口。', emits: 'recover', labelTag: 'perturb' }],
  render: renderNonceView,
  narrative: nonceNarrative,
  codeTrace: nonceCodeTrace,
  checkpoints: [{ id: 'nonce-order-valid', label: 'Nonce 顺序有效', evaluate: (state) => nonceValid(state as NonceState) }],
};
