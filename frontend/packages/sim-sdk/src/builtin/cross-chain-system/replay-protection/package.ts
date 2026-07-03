// 本文件装配跨链消息重放防护仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialReplayState, reduceReplayEvent, replayProtected } from './kernel';
import { replayCodeTrace, replayNarrative } from './trace';
import { renderReplayView } from './view';
import type { ReplayState } from './model';

export const replayProtectionSimulation: SimPackage<ReplayState> = {
  meta: { code: 'builtin__cross-replay-protection', name: '跨链消息重放防护推演', category: 'cross-chain-system', version: '1.0.0', compute: 'frontend', summary: '完整推演跨链消息域分离、nonce 分配、已执行集合、重放拒绝和协议版本轮换。', learningObjectives: ['理解跨链消息为什么需要域分离', '掌握 nonce 与已执行集合的关系', '观察有效证明为何仍可能被重放拒绝'], scaleLimit: { nodes: 48, maxTick: 120, maxEvents: 220 } },
  initState: createInitialReplayState,
  reducer: reduceReplayEvent,
  interactions: [{ id: 'advance', kind: 'button', label: '推进阶段', description: '推进重放防护流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '重放消息', description: '再次提交已经执行过的消息。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '轮换版本', description: '升级 domain 并保留历史执行记录。', emits: 'recover', labelTag: 'perturb' }],
  render: renderReplayView,
  narrative: replayNarrative,
  codeTrace: replayCodeTrace,
  checkpoints: [{ id: 'replay-protected', label: '跨链重放已拒绝', evaluate: (state) => replayProtected(state as ReplayState) }],
};
