// 本文件聚合共识算法内置仿真包,每个算法的状态机实现位于独立文件。

import type { SimPackage } from '../../types';
import { hotstuffSimulation } from './hotstuff/package';
import { pbftSimulation } from './pbft/package';
import { posSimulation } from './pos/package';
import { powSimulation } from './pow/package';
import { raftSimulation } from './raft/package';

export const consensusSimulations: SimPackage[] = [
  pbftSimulation as unknown as SimPackage,
  powSimulation as unknown as SimPackage,
  raftSimulation as unknown as SimPackage,
  posSimulation as unknown as SimPackage,
  hotstuffSimulation as unknown as SimPackage,
];
