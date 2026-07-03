// 本文件装配延迟丢包与可靠重传仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { allDelivered, createInitialLatencyLossState, reduceLatencyLossEvent } from './kernel';
import { latencyLossCodeTrace, latencyLossNarrative } from './trace';
import { renderLatencyLossView } from './view';
import type { LatencyLossState } from './model';

export const latencyLossSimulation: SimPackage<LatencyLossState> = {
  meta: {
    code: 'builtin__network-latency-loss',
    name: '延迟丢包与重传推演',
    category: 'network',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演可靠传输中的发送窗口、网络延迟、丢包检测、重传、退避与窗口恢复。',
    learningObjectives: ['理解延迟和丢包的差异', '掌握超时重传流程', '观察窗口退避如何保护网络'],
    scaleLimit: { nodes: 64, maxTick: 140, maxEvents: 240 },
  },
  initState: createInitialLatencyLossState,
  reducer: reduceLatencyLossEvent,
  interactions: commonAlgorithmInteractions('packet'),
  render: renderLatencyLossView,
  narrative: latencyLossNarrative,
  codeTrace: latencyLossCodeTrace,
  checkpoints: [{ id: 'latency-loss-delivered', label: '丢包后可靠送达', evaluate: (state) => allDelivered(state as LatencyLossState) }],
};
