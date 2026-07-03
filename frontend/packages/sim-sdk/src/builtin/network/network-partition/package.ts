// 本文件装配网络分区与恢复仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { createInitialPartitionState, partitionMerged, reducePartitionEvent } from './kernel';
import { partitionCodeTrace, partitionNarrative } from './trace';
import { renderPartitionView } from './view';
import type { PartitionState } from './model';

export const networkPartitionSimulation: SimPackage<PartitionState> = {
  meta: {
    code: 'builtin__network-partition-recovery',
    name: '网络分区与恢复推演',
    category: 'network',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演网络分区的拓扑切割、分区内同步、跨区阻断、链路恢复与状态合并。',
    learningObjectives: ['理解分区导致的可达性变化', '区分局部一致和全局一致', '掌握分区恢复后的合并步骤'],
    scaleLimit: { nodes: 96, maxTick: 140, maxEvents: 240 },
  },
  initState: createInitialPartitionState,
  reducer: reducePartitionEvent,
  interactions: commonAlgorithmInteractions('partition-node'),
  render: renderPartitionView,
  narrative: partitionNarrative,
  codeTrace: partitionCodeTrace,
  checkpoints: [{ id: 'partition-merged', label: '分区恢复并完成合并', evaluate: (state) => partitionMerged(state as PartitionState) }],
};
