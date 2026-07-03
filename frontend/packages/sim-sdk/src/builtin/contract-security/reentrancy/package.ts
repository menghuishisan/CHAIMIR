// 本文件装配重入攻击与防护仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialReentrancyState, reduceReentrancyEvent, reentrancyBlocked } from './kernel';
import { reentrancyCodeTrace, reentrancyNarrative } from './trace';
import { renderReentrancyView } from './view';
import type { ReentrancyState } from './model';

export const reentrancySimulation: SimPackage<ReentrancyState> = {
  meta: { code: 'builtin__security-reentrancy', name: '重入攻击与防护推演', category: 'contract-security', version: '1.0.0', compute: 'frontend', summary: '完整推演重入攻击的合法存款、提款外部调用、fallback 回调重入、余额错序更新和重入锁修复。', learningObjectives: ['理解外部调用早于状态更新的风险', '掌握重入攻击调用栈', '观察重入锁和先改状态如何防护'], scaleLimit: { nodes: 64, maxTick: 120, maxEvents: 220 } },
  initState: createInitialReentrancyState,
  reducer: reduceReentrancyEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择参与方', description: '查看调用栈中的参与方状态。', emits: 'select', target: 'element', elementFilter: 'security-actor' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进重入攻击流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '触发回调重入', description: '让攻击合约在回调中重复提款。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '启用重入锁', description: '按安全顺序修复提款流程。', emits: 'recover', labelTag: 'perturb' }],
  render: renderReentrancyView,
  narrative: reentrancyNarrative,
  codeTrace: reentrancyCodeTrace,
  checkpoints: [{ id: 'reentrancy-blocked', label: '重入已被阻断', evaluate: (state) => reentrancyBlocked(state as ReentrancyState) }],
};
