// 本文件装配哈希链篡改扩散仿真包,算法内核、视图和追踪分别由同目录文件维护。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { createInitialHashChainState, hashChainValid, reduceHashChainEvent } from './kernel';
import { hashChainCodeTrace, hashChainNarrative } from './trace';
import { renderHashChainView } from './view';
import type { HashChainState } from './model';

export const hashChainSimulation: SimPackage<HashChainState> = {
  meta: {
    code: 'builtin__crypto-hash-chain',
    name: '哈希链篡改扩散推演',
    category: 'cryptography',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演哈希输入规范化、摘要计算、父哈希串联、篡改检测与链式重算修复。',
    learningObjectives: ['理解哈希雪崩效应', '掌握父哈希链式依赖', '观察篡改为什么会向后传播'],
    scaleLimit: { nodes: 80, maxTick: 120, maxEvents: 200 },
  },
  initState: createInitialHashChainState,
  reducer: reduceHashChainEvent,
  interactions: commonAlgorithmInteractions('hash-record'),
  render: renderHashChainView,
  narrative: hashChainNarrative,
  codeTrace: hashChainCodeTrace,
  checkpoints: [{ id: 'hash-chain-valid', label: '哈希链校验通过', evaluate: (state) => hashChainValid(state as HashChainState) }],
};
