// 本文件定义 DHT 异或路由仿真的领域模型和阶段元数据。

import type { SimState } from '../../../types';
import type { NetworkMessageView, NetworkNodeView } from '../networkView';

export interface DhtPeer extends NetworkNodeView {
  nodeId: number;
  bucket: number;
  queried: boolean;
  inShortlist: boolean;
  closest: boolean;
  polluted: boolean;
  hasValue: boolean;
  returnedPeers: number[];
}

export interface DhtState extends SimState {
  phaseIndex: number;
  lookupKey: number;
  alpha: number;
  bucketSize: number;
  peers: DhtPeer[];
  shortlist: string[];
  messages: NetworkMessageView[];
  hops: number;
  foundValue: boolean;
  lastTransition: string;
}

export const dhtPhases = [
  { id: 'id-space', label: '映射 ID 空间', detail: '哈希为节点 ID', effect: '节点和数据 key 都被映射到同一个 ID 空间。', reason: 'DHT 用距离度量而不是物理拓扑决定查找路径。' },
  { id: 'bucket', label: '维护 K 桶', detail: '按距离分桶', effect: '节点按异或距离维护不同范围的邻居。', reason: 'K 桶让路由表在远近距离上都有代表节点。' },
  { id: 'distance', label: '计算异或距离', detail: '选择更近节点', effect: '查询方根据异或距离挑选更接近 key 的节点。', reason: '异或距离提供可单调逼近目标的路由方向。' },
  { id: 'query', label: '迭代查询', detail: '逐跳询问更近节点', effect: '每轮请求更近节点,直到没有更近候选或找到值。', reason: '迭代查询把查找控制权留在发起者,便于超时和重试。' },
  { id: 'repair', label: '修复污染路由', detail: '剔除错误候选', effect: '发现返回远离目标的节点后降低信任并改问备用节点。', reason: '路由污染会拖慢或劫持查找,必须可检测。' },
] as const;
