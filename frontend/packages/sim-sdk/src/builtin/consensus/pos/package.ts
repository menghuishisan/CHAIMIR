// 本文件把 PoS 内核、视图、叙事和检查点装配为 SimPackage 入口。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { createInitialPosState, posSlashingHandled, posTwoThirdsFinality, reducePosEvent } from './kernel';
import type { PosState } from './model';
import { posCodeTrace, posNarrative } from './trace';
import { renderPosView } from './view';

/**
 * posSimulation 将 PoS 权益证明和最终性暴露给 M4 运行时。
 */
export const posSimulation: SimPackage<PosState> = {
  meta: {
    code: 'builtin__pos-finality',
    name: 'PoS 权益证明与最终性推演',
    category: 'consensus',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演 PoS 随机选主、区块提议、权益加权见证、检查点证明、最终性与双签罚没。',
    learningObjectives: ['理解权益权重与随机选主', '掌握三分之二权益见证阈值', '观察双签为何会被罚没'],
    scaleLimit: { nodes: 128, maxTick: 180, maxEvents: 320 },
  },
  initState: createInitialPosState,
  reducer: reducePosEvent,
  interactions: commonAlgorithmInteractions('validator'),
  render: renderPosView,
  narrative: posNarrative,
  codeTrace: posCodeTrace,
  checkpoints: [
    { id: 'pos-two-thirds-finality', label: '三分之二权益最终性', evaluate: (state) => posTwoThirdsFinality(state as PosState) },
    { id: 'pos-slashing', label: '双签罚没处理', evaluate: (state) => posSlashingHandled(state as PosState) },
  ],
};

