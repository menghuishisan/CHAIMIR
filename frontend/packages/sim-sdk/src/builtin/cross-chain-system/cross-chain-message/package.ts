// 本文件装配跨链消息生命周期仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialCrossMessageState, messageExecuted, reduceCrossMessageEvent } from './kernel';
import { crossMessageCodeTrace, crossMessageNarrative } from './trace';
import { renderCrossMessageView } from './view';
import type { CrossChainMessageState } from './model';

export const crossChainMessageSimulation: SimPackage<CrossChainMessageState> = {
  meta: { code: 'builtin__cross-message-lifecycle', name: '跨链消息生命周期推演', category: 'cross-chain-system', version: '1.0.0', compute: 'frontend', summary: '完整推演跨链消息从源链锁定、消息构造、中继提交、目标链验证到执行回执的生命周期。', learningObjectives: ['理解跨链消息必须绑定源链事件', '区分中继传递和目标链验证', '观察回执如何形成终态'], scaleLimit: { nodes: 64, maxTick: 120, maxEvents: 220 } },
  initState: createInitialCrossMessageState,
  reducer: reduceCrossMessageEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择链或中继', description: '查看跨链消息当前状态。', emits: 'select', target: 'element', elementFilter: 'cross-actor' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进跨链消息流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '中继丢失', description: '模拟中继未提交消息。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '重新中继', description: '重新提交消息和证明。', emits: 'recover', labelTag: 'perturb' }],
  render: renderCrossMessageView,
  narrative: crossMessageNarrative,
  codeTrace: crossMessageCodeTrace,
  checkpoints: [{ id: 'cross-message-executed', label: '跨链消息已验证执行', evaluate: (state) => messageExecuted(state as CrossChainMessageState) }],
};
