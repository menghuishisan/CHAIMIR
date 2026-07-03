// 本文件定义 P2P 节点发现仿真的领域模型和阶段元数据。

import type { SimState } from '../../../types';
import type { NetworkMessageView, NetworkNodeView } from '../networkView';

export interface DiscoveryAddress {
  peerId: string;
  networkId: string;
  protocolVersion: number;
  score: number;
  source: string;
}

export interface DiscoveryPeer extends NetworkNodeView {
  connected: boolean;
  knownAddrs: DiscoveryAddress[];
  healthy: boolean;
  malicious: boolean;
  banned: boolean;
  protocolVersion: number;
  networkId: string;
  failedHandshakeReason?: string;
}

export interface DiscoveryState extends SimState {
  phaseIndex: number;
  localNetworkId: string;
  minProtocolVersion: number;
  peers: DiscoveryPeer[];
  messages: NetworkMessageView[];
  addressBook: DiscoveryAddress[];
  handshakeCount: number;
  bannedPeerIds: string[];
  lastTransition: string;
}

export const discoveryPhases = [
  { id: 'bootstrap', label: '连接引导节点', detail: '获取入口地址', effect: '新节点先连接可信引导节点获取网络入口。', reason: '完全陌生节点需要至少一个已知入口才能加入网络。' },
  { id: 'addr', label: '交换地址簿', detail: '返回候选节点', effect: '引导节点返回已知 peer 地址列表。', reason: '地址簿扩散让节点逐步摆脱单一入口依赖。' },
  { id: 'handshake', label: '执行握手', detail: '校验网络和版本', effect: '节点与候选 peer 校验网络标识、协议版本和能力。', reason: '握手能过滤错误网络和不兼容节点。' },
  { id: 'probe', label: '健康探测', detail: '周期 ping', effect: '节点通过 ping/pong 判断连接质量。', reason: '发现并剔除失联节点才能维持拓扑活性。' },
  { id: 'ban', label: '剔除恶意节点', detail: '写入本地黑名单', effect: '对返回错误地址或握手异常的 peer 降权或剔除。', reason: '发现机制必须抵抗恶意地址投毒。' },
] as const;
