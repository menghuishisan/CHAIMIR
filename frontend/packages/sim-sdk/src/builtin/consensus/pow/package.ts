// 本文件把 PoW 内核、视图、叙事和检查点装配为 SimPackage 入口。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { createInitialPowState, powForkChoiceValid, powWorkValid, reducePowEvent } from './kernel';
import type { PowState } from './model';
import { powCodeTrace, powNarrative } from './trace';
import { renderPowView } from './view';

/**
 * powSimulation 将 PoW 最长链共识暴露给 M4 运行时。
 */
export const powSimulation: SimPackage<PowState> = {
  meta: {
    code: 'builtin__pow-longest-chain',
    name: 'PoW 最长链共识推演',
    category: 'consensus',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演 PoW 从交易打包、nonce 搜索、区块广播验证到最长链收敛与难度调整的流程。',
    learningObjectives: ['理解工作量证明和难度目标', '观察临时分叉如何按累计工作量收敛', '分析自私挖矿对最终性的影响'],
    scaleLimit: { nodes: 128, maxTick: 180, maxEvents: 320 },
  },
  initState: createInitialPowState,
  reducer: reducePowEvent,
  interactions: commonAlgorithmInteractions('miner'),
  render: renderPowView,
  narrative: powNarrative,
  codeTrace: powCodeTrace,
  checkpoints: [
    { id: 'pow-work-valid', label: '工作量证明有效', evaluate: (state) => powWorkValid(state as PowState) },
    { id: 'pow-fork-choice', label: '按累计工作量选链', evaluate: (state) => powForkChoiceValid(state as PowState) },
  ],
};

