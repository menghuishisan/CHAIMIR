// 本文件定义网络分区与恢复仿真的领域模型和阶段元数据。

import type { SimState } from '../../../types';
import type { NetworkMessageView, NetworkNodeView } from '../networkView';

export interface PartitionNode extends NetworkNodeView {
  group: 'left' | 'right';
  reachable: boolean;
  syncedVersion: number;
  localWrites: number;
}

export interface PartitionLink {
  id: string;
  from: string;
  to: string;
  crossRegion: boolean;
  cut: boolean;
}

export interface PartitionState extends SimState {
  phaseIndex: number;
  partitionActive: boolean;
  nodes: PartitionNode[];
  links: PartitionLink[];
  messages: NetworkMessageView[];
  samples: Array<{ x: number; coverage: number; risk: number; latency: number }>;
  lastTransition: string;
}

export const partitionPhases = [
  { id: 'topology', label: '识别拓扑边界', detail: '标记跨区连接', effect: '系统识别哪些链路连接不同网络区域。', reason: '分区判断需要先知道拓扑中的割边。' },
  { id: 'cut', label: '切断跨区链路', detail: '阻断跨区消息', effect: '跨区消息无法送达,两侧只能在本区传播。', reason: '网络分区会让全局状态出现暂时分歧。' },
  { id: 'local-sync', label: '分区内同步', detail: '各自达成本地一致', effect: '每个分区内部继续同步自己的最新版本。', reason: '局部可用不等于全局一致。' },
  { id: 'heal', label: '恢复跨区链路', detail: '重新连通', effect: '网络恢复后跨区消息重新可达。', reason: '恢复阶段需要处理两个分区产生的状态差异。' },
  { id: 'merge', label: '合并状态', detail: '选择权威版本', effect: '节点按版本或共识规则合并分歧状态。', reason: '分区恢复后必须显式收敛,不能假设自动一致。' },
] as const;
