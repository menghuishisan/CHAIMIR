// 本文件聚合共识算法内置仿真包,每个算法的状态机实现位于独立文件。

import type { SimPackage } from '../../types';
import { ethereumPosFinalitySimulation } from './ethereum-pos-finality/package';
import { hotstuffSimulation } from './hotstuff/package';
import { pbftSimulation } from './pbft/package';
import { posSimulation } from './pos/package';
import { powSimulation } from './pow/package';
import { raftSimulation } from './raft/package';
import { tendermintRoundsSimulation } from './tendermint-rounds/package';

export const consensusSimulations: SimPackage[] = [
  pbftSimulation,
  powSimulation,
  raftSimulation,
  posSimulation,
  ethereumPosFinalitySimulation,
  tendermintRoundsSimulation,
  hotstuffSimulation,
];
