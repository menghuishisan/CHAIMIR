// 本文件装配状态快照与回滚仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialSnapshotState, reduceSnapshotEvent, snapshotValid } from './kernel';
import { snapshotCodeTrace, snapshotNarrative } from './trace';
import { renderSnapshotView } from './view';
import type { SnapshotState } from './model';

export const stateSnapshotSimulation: SimPackage<SnapshotState> = {
  meta: { code: 'builtin__data-state-snapshot', name: '状态快照与回滚推演', category: 'data-structure', version: '1.0.0', compute: 'frontend', summary: '完整推演状态快照的同高收集、根摘要计算、增量变更、异常回滚和快照根校验。', learningObjectives: ['理解快照根如何承诺状态', '掌握增量变更与回滚关系', '观察执行失败如何恢复状态'], scaleLimit: { nodes: 80, maxTick: 120, maxEvents: 220 } },
  initState: createInitialSnapshotState,
  reducer: reduceSnapshotEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择账户', description: '选择账户查看快照状态。', emits: 'select', target: 'element', elementFilter: 'account' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进快照流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '写入异常变更', description: '模拟执行失败前的脏写入。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '回滚快照', description: '按快照恢复账户状态。', emits: 'recover', labelTag: 'perturb' }],
  render: renderSnapshotView,
  narrative: snapshotNarrative,
  codeTrace: snapshotCodeTrace,
  checkpoints: [{ id: 'snapshot-root-valid', label: '快照根恢复一致', evaluate: (state) => snapshotValid(state as SnapshotState) }],
};
