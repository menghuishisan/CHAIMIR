// 本文件装配 P2P 节点发现仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { createInitialDiscoveryState, healthyDiscovery, reduceDiscoveryEvent } from './kernel';
import { discoveryCodeTrace, discoveryNarrative } from './trace';
import { renderDiscoveryView } from './view';
import type { DiscoveryState } from './model';

export const p2pDiscoverySimulation: SimPackage<DiscoveryState> = {
  meta: {
    code: 'builtin__network-p2p-discovery',
    name: 'P2P 节点发现推演',
    category: 'network',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演 P2P 网络从引导节点接入、地址交换、握手校验、健康探测到恶意节点剔除的过程。',
    learningObjectives: ['理解节点如何从零加入网络', '掌握地址簿和握手的安全边界', '观察地址投毒如何被剔除'],
    scaleLimit: { nodes: 96, maxTick: 140, maxEvents: 240 },
  },
  initState: createInitialDiscoveryState,
  reducer: reduceDiscoveryEvent,
  interactions: commonAlgorithmInteractions('p2p-peer'),
  render: renderDiscoveryView,
  narrative: discoveryNarrative,
  codeTrace: discoveryCodeTrace,
  checkpoints: [{ id: 'p2p-discovery-healthy', label: '节点发现拓扑健康', evaluate: (state) => healthyDiscovery(state as DiscoveryState) }],
};
