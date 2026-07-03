// 本文件把 HotStuff 内核、视图、叙事和检查点装配为 SimPackage 入口。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { createInitialHotStuffState, hotstuffThreeChainCommitted, hotstuffTimeoutRecovered, reduceHotStuffEvent } from './kernel';
import type { HotStuffState } from './model';
import { hotstuffCodeTrace, hotstuffNarrative } from './trace';
import { renderHotStuffView } from './view';

/**
 * hotstuffSimulation 将 HotStuff 链式 BFT 暴露给 M4 运行时。
 */
export const hotstuffSimulation: SimPackage<HotStuffState> = {
  meta: {
    code: 'builtin__hotstuff-chained-bft',
    name: 'HotStuff 链式 BFT 推演',
    category: 'consensus',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演 HotStuff 新视图、高 QC 提案、安全投票、QC 聚合、三链提交和 pacemaker 超时换主。',
    learningObjectives: ['理解 high QC 与锁规则', '掌握 BFT 法定人数投票聚合成 QC', '观察三链提交和超时换主如何保证安全与活性'],
    scaleLimit: { nodes: 96, maxTick: 160, maxEvents: 300 },
  },
  initState: createInitialHotStuffState,
  reducer: reduceHotStuffEvent,
  interactions: commonAlgorithmInteractions('hotstuff-replica'),
  render: renderHotStuffView,
  narrative: hotstuffNarrative,
  codeTrace: hotstuffCodeTrace,
  checkpoints: [
    { id: 'hotstuff-three-chain', label: '三链提交成立', evaluate: (state) => hotstuffThreeChainCommitted(state as HotStuffState) },
    { id: 'hotstuff-timeout-recovery', label: '超时换主恢复', evaluate: (state) => hotstuffTimeoutRecovered(state as HotStuffState) },
  ],
};
