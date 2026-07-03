// 本文件把 PBFT 协议内核、过程化可视化、叙事和检查点装配为 SimPackage。

import type { InteractionDef, SimPackage } from '../../../types';
import { createInitialPbftState, pbftCheckpointStability, pbftSafetyCheckpoint, pbftViewChangeCheckpoint, reducePbftEvent } from './kernel';
import type { PbftState } from './model';
import { renderPbftView } from './view';
import { pbftCodeTrace, pbftNarrative } from './trace';

const pbftInteractions: InteractionDef[] = [
  { id: 'select', kind: 'select-element', label: '选择对象', description: '选择副本、消息或证书,查看它在 PBFT 当前过程中的状态。', emits: 'select', target: 'element', elementFilter: 'participant' },
  { id: 'advance', kind: 'button', label: '推进过程', description: '按 PBFT 协议规则推进一个确定性过程单元。', emits: 'advance', labelTag: 'normal' },
  { id: 'attack', kind: 'button', label: '注入双提议', description: '让当前主节点发送冲突摘要,观察正确副本如何拒绝并保留证据。', emits: 'attack', labelTag: 'attack', params: [{ name: 'confirmed', label: '确认执行', type: 'boolean', default: false }] },
  { id: 'recover', kind: 'button', label: '执行视图切换', description: '收集视图切换证书,由新主节点继承安全摘要并恢复推进。', emits: 'recover', labelTag: 'perturb' },
];

/**
 * pbftSimulation 将完整 PBFT 协议过程暴露给 M4 运行时。
 */
export const pbftSimulation: SimPackage<PbftState> = {
  meta: {
    code: 'builtin__pbft-consensus',
    name: 'PBFT 三阶段共识推演',
    category: 'consensus',
    version: '1.0.0',
    compute: 'frontend',
    summary: '以协议内核推演 PBFT 请求、预准备、准备、提交、回复、检查点和视图切换,可观察消息飞行、证书形成和安全条件。',
    learningObjectives: ['理解 PBFT 三阶段投票', '掌握 2f+1 法定人数', '观察拜占庭主节点双提议与视图切换恢复'],
    scaleLimit: { nodes: 96, maxTick: 120, maxEvents: 240 },
  },
  initState: createInitialPbftState,
  reducer: reducePbftEvent,
  interactions: pbftInteractions,
  render: renderPbftView,
  narrative: pbftNarrative,
  codeTrace: pbftCodeTrace,
  checkpoints: [
    { id: 'pbft-safety', label: 'PBFT 安全提交', evaluate: (state) => pbftSafetyCheckpoint(state as PbftState) },
    { id: 'pbft-view-change', label: '视图切换处理', evaluate: (state) => pbftViewChangeCheckpoint(state as PbftState) },
    { id: 'pbft-checkpoint', label: '稳定检查点', evaluate: (state) => pbftCheckpointStability(state as PbftState) },
  ],
};
