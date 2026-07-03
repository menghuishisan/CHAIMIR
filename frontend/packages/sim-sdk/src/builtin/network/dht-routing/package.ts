// 本文件装配 DHT 异或路由仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { createInitialDhtState, lookupFound, reduceDhtEvent } from './kernel';
import { dhtCodeTrace, dhtNarrative } from './trace';
import { renderDhtView } from './view';
import type { DhtState } from './model';

export const dhtRoutingSimulation: SimPackage<DhtState> = {
  meta: {
    code: 'builtin__network-dht-routing',
    name: 'DHT 异或路由推演',
    category: 'network',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演 DHT 节点 ID 空间、K 桶维护、异或距离选择、迭代查询与污染路由修复。',
    learningObjectives: ['理解 DHT 为什么按异或距离路由', '掌握 K 桶如何覆盖 ID 空间', '观察路由污染如何影响查找'],
    scaleLimit: { nodes: 96, maxTick: 140, maxEvents: 240 },
  },
  initState: createInitialDhtState,
  reducer: reduceDhtEvent,
  interactions: commonAlgorithmInteractions('dht-peer'),
  render: renderDhtView,
  narrative: dhtNarrative,
  codeTrace: dhtCodeTrace,
  checkpoints: [{ id: 'dht-lookup-found', label: 'DHT 查找成功', evaluate: (state) => lookupFound(state as DhtState) }],
};
