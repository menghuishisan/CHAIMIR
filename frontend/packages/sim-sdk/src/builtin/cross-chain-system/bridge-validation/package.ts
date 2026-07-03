// 本文件装配跨链桥证明验证仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { bridgeValid, createInitialBridgeState, reduceBridgeEvent } from './kernel';
import { bridgeCodeTrace, bridgeNarrative } from './trace';
import { renderBridgeView } from './view';
import type { BridgeState } from './model';

export const bridgeValidationSimulation: SimPackage<BridgeState> = {
  meta: { code: 'builtin__cross-bridge-validation', name: '跨链桥证明验证推演', category: 'cross-chain-system', version: '1.0.0', compute: 'frontend', summary: '完整推演跨链桥锁仓证明、轻客户端同步、包含证明验证、目标链铸造和赎回闭环。', learningObjectives: ['理解桥为什么要验证源链证明', '掌握轻客户端同步作用', '观察错误证明如何被拒绝'], scaleLimit: { nodes: 48, maxTick: 120, maxEvents: 220 } },
  initState: createInitialBridgeState,
  reducer: reduceBridgeEvent,
  interactions: [{ id: 'advance', kind: 'button', label: '推进阶段', description: '推进桥验证流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '提交错误证明', description: '提交无法验证的锁仓证明。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '同步轻客户端', description: '更新源链头后重新验证。', emits: 'recover', labelTag: 'perturb' }],
  render: renderBridgeView,
  narrative: bridgeNarrative,
  codeTrace: bridgeCodeTrace,
  checkpoints: [{ id: 'bridge-proof-valid', label: '桥证明验证通过', evaluate: (state) => bridgeValid(state as BridgeState) }],
};
