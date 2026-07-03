// 本文件聚合网络传播内置仿真包,每个网络机制的状态机实现位于独立文件。

import type { SimPackage } from '../../types';
import { dhtRoutingSimulation } from './dht-routing/package';
import { gossipPropagationSimulation } from './gossip-propagation/package';
import { latencyLossSimulation } from './latency-loss/package';
import { networkPartitionSimulation } from './network-partition/package';
import { p2pDiscoverySimulation } from './p2p-discovery/package';

export const networkSimulations: SimPackage[] = [
  p2pDiscoverySimulation as unknown as SimPackage,
  gossipPropagationSimulation as unknown as SimPackage,
  dhtRoutingSimulation as unknown as SimPackage,
  networkPartitionSimulation as unknown as SimPackage,
  latencyLossSimulation as unknown as SimPackage,
];
