// 本文件定义 Gossip 传播仿真的领域模型和阶段元数据。

import type { SimState } from '../../../types';
import type { NetworkMessageView, NetworkNodeView } from '../networkView';

export interface GossipPeer extends NetworkNodeView {
  informed: boolean;
  duplicateCount: number;
  polluted: boolean;
  seenMessageIds: string[];
  neighbors: string[];
  activeSender: boolean;
}

export interface GossipState extends SimState {
  phaseIndex: number;
  fanout: number;
  messageId: string;
  round: number;
  frontier: string[];
  peers: GossipPeer[];
  messages: NetworkMessageView[];
  samples: Array<{ x: number; coverage: number; risk: number; latency: number }>;
  lastTransition: string;
}

export const gossipPhases = [
  { id: 'seed', label: '种子节点发布消息', detail: '选定初始源', effect: '种子节点获得待传播消息并成为第一轮传播源。', reason: 'Gossip 从少量源节点扩散,不依赖中心广播。' },
  { id: 'fanout', label: '选择扇出邻居', detail: '按 fanout 选择目标', effect: '每个已知节点选择固定数量邻居转发消息。', reason: '扇出决定传播速度、带宽成本和重复消息比例。' },
  { id: 'spread', label: '逐轮扩散', detail: '邻居继续转发', effect: '新收到消息的节点加入下一轮转发。', reason: '多轮随机扩散让消息以较低成本覆盖网络。' },
  { id: 'dedupe', label: '重复抑制', detail: '丢弃已见消息', effect: '节点对已处理消息只计数不再执行。', reason: '重复抑制防止 Gossip 在密集网络中形成广播风暴。' },
  { id: 'converge', label: '覆盖收敛', detail: '检查覆盖率', effect: '当大多数节点都已收到消息,传播进入收敛态。', reason: '覆盖率是 Gossip 正确性的核心观察指标。' },
] as const;
