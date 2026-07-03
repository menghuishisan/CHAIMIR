// 本文件装配零知识证明仿真包,算法内核、视图和追踪分别由同目录文件维护。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { createInitialZkProofState, reduceZkProofEvent, zkProofValid } from './kernel';
import { zkProofCodeTrace, zkProofNarrative } from './trace';
import { renderZkProofView } from './view';
import type { ZkState } from './model';

export const zkProofSimulation: SimPackage<ZkState> = {
  meta: {
    code: 'builtin__crypto-zk-proof',
    name: '零知识证明交互流程推演',
    category: 'cryptography',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演零知识证明的见证、承诺、挑战、响应、验证等式与多轮可靠性放大。',
    learningObjectives: ['理解承诺-挑战-响应结构', '区分完整性和零知识性', '观察作弊证明为什么会被挑战暴露'],
    scaleLimit: { nodes: 48, maxTick: 140, maxEvents: 220 },
  },
  initState: createInitialZkProofState,
  reducer: reduceZkProofEvent,
  interactions: commonAlgorithmInteractions('crypto-actor'),
  render: renderZkProofView,
  narrative: zkProofNarrative,
  codeTrace: zkProofCodeTrace,
  checkpoints: [{ id: 'zk-proof-valid', label: '零知识约束验证通过', evaluate: (state) => zkProofValid(state as ZkState) }],
};
