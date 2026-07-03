// 本文件装配 Gossip 消息传播仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { coverageCheckpoint, createInitialGossipState, reduceGossipEvent } from './kernel';
import { gossipCodeTrace, gossipNarrative } from './trace';
import { renderGossipView } from './view';
import type { GossipState } from './model';

export const gossipPropagationSimulation: SimPackage<GossipState> = {
  meta: {
    code: 'builtin__network-gossip-propagation',
    name: 'Gossip 消息传播推演',
    category: 'network',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演 Gossip 的种子广播、扇出传播、重复抑制、覆盖收敛和污染消息隔离。',
    learningObjectives: ['理解 Gossip 为什么能低成本覆盖网络', '观察 fanout 对速度和重复消息的影响', '掌握污染消息如何被隔离'],
    scaleLimit: { nodes: 120, maxTick: 160, maxEvents: 280 },
  },
  initState: createInitialGossipState,
  reducer: reduceGossipEvent,
  interactions: commonAlgorithmInteractions('gossip-peer'),
  render: renderGossipView,
  narrative: gossipNarrative,
  codeTrace: gossipCodeTrace,
  checkpoints: [{ id: 'gossip-coverage', label: 'Gossip 覆盖率达标', evaluate: (state) => coverageCheckpoint(state as GossipState) }],
};
