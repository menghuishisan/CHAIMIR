// 本文件实现 P2P 引导、地址交换、握手校验、健康探测和恶意地址剔除内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { processNetworkMessage, refreshNetworkMessages, type NetworkMessageView } from '../networkView';
import { discoveryPhases, type DiscoveryAddress, type DiscoveryPeer, type DiscoveryState } from './model';
import { traceLinesForDiscovery } from './trace';

/**
 * createInitialDiscoveryState 创建引导节点、候选节点和本地网络约束。
 */
export function createInitialDiscoveryState(_params: SimInitParams, _seed: number): DiscoveryState {
  const localNetworkId = 'chaimir-main';
  const peers = ['Boot', 'A', 'B', 'C', 'D'].map<DiscoveryPeer>((label, index) => {
    const peerId = `p2p-${label.toLowerCase()}`;
    return {
      id: peerId,
      label: index === 0 ? '引导节点' : `节点 ${label}`,
      role: 'p2p-peer',
      status: index === 0 ? 'active' : 'idle',
      value: index === 0 ? '入口' : '候选',
      connected: index === 0,
      knownAddrs: [],
      healthy: true,
      malicious: false,
      banned: false,
      protocolVersion: index === 3 ? 1 : 2,
      networkId: localNetworkId,
    };
  });
  return finalizeDiscoveryState({
    tick: 0,
    phase: discoveryPhases[0].label,
    phaseIndex: 0,
    localNetworkId,
    minProtocolVersion: 2,
    peers,
    messages: [],
    addressBook: [],
    handshakeCount: 0,
    bannedPeerIds: [],
    lastTransition: 'bootstrap',
    explanation: explainDiscoveryPhase(0),
    metrics: {},
    checkpointValues: {},
  });
}

/**
 * reduceDiscoveryEvent 是 P2P 发现仿真包唯一事件入口。
 */
export function reduceDiscoveryEvent(state: DiscoveryState, event: SimEvent, _context: ReducerContext): DiscoveryState {
  if (event.type === 'select') return finalizeDiscoveryState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeDiscoveryState(poisonAddressBook(state));
  if (event.type === 'recover') return finalizeDiscoveryState(banInvalidPeers(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeDiscoveryState(advanceDiscovery(state, event));
  return state;
}

/**
 * advanceDiscovery 按发现流程推进一个过程单元。
 */
export function advanceDiscovery(state: DiscoveryState, event: SimEvent): DiscoveryState {
  const phaseIndex = Math.min(discoveryPhases.length - 1, state.phaseIndex + 1);
  const next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick };
  if (phaseIndex === 1) return exchangeAddresses(next);
  if (phaseIndex === 2) return performHandshakes(next);
  if (phaseIndex === 3) return probePeers(next);
  if (phaseIndex === 4) return banInvalidPeers(next);
  return { ...next, lastTransition: discoveryPhases[phaseIndex].id };
}

/**
 * healthyDiscovery 输出发现健康检查点。
 */
export function healthyDiscovery(state: DiscoveryState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.healthy);
  return { achieved, answer: { connected: state.metrics.connected, banned: state.metrics.banned }, explanation: achieved ? '节点发现拓扑已过滤异常地址并保持可用。' : '拓扑仍有连接不足或异常节点未剔除。' };
}

/**
 * finalizeDiscoveryState 刷新节点状态、消息进度、指标和代码追踪。
 */
export function finalizeDiscoveryState(state: DiscoveryState): DiscoveryState {
  const connected = state.peers.filter((peer) => peer.connected && peer.healthy && !peer.banned).length;
  const risky = state.peers.some((peer) => (peer.malicious || peer.failedHandshakeReason) && !peer.banned);
  const banned = state.peers.filter((peer) => peer.banned).length;
  return {
    ...state,
    phase: discoveryPhases[state.phaseIndex].label,
    peers: state.peers.map((peer) => ({ ...peer, status: peer.banned ? 'warning' : peer.malicious || peer.failedHandshakeReason ? 'danger' : peer.connected ? 'success' : peer.id === 'p2p-boot' ? 'active' : 'idle', value: peer.banned ? '已剔除' : peer.connected ? '已连接' : peer.failedHandshakeReason ?? '候选' })),
    messages: refreshNetworkMessages(state.messages, state.tick, (message) => message.detail ?? `${message.label} 正在节点发现流程中传播。`),
    explanation: explainDiscoveryPhase(state.phaseIndex),
    metrics: { result: connected >= 3 && !risky ? '拓扑可用' : '需要过滤', risk: risky ? 76 : 12, connected, banned },
    checkpointValues: { healthy: connected >= 3 && !risky },
    _trace: { triggeredLines: traceLinesForDiscovery(state.lastTransition), variables: { handshakeCount: state.handshakeCount, connected, banned }, executionPath: `p2p-discovery/${state.lastTransition}` },
  };
}

/**
 * exchangeAddresses 从引导节点返回候选地址并做去重评分。
 */
function exchangeAddresses(state: DiscoveryState): DiscoveryState {
  const addresses = state.peers
    .filter((peer) => peer.id !== 'p2p-boot')
    .map<DiscoveryAddress>((peer) => ({ peerId: peer.id, networkId: peer.networkId, protocolVersion: peer.protocolVersion, score: peer.malicious ? -20 : peer.protocolVersion >= state.minProtocolVersion ? 80 : 30, source: 'p2p-boot' }));
  return { ...state, lastTransition: 'addr', addressBook: dedupeAddresses(addresses), messages: state.messages.concat(broadcast(state, 'p2p-boot', '地址簿', '引导节点返回候选地址,本地节点会继续去重和评分。')) };
}

/**
 * performHandshakes 校验网络标识、协议版本和黑名单状态。
 */
function performHandshakes(state: DiscoveryState): DiscoveryState {
  const addressByPeer = new Map(state.addressBook.map((address) => [address.peerId, address]));
  const peers = state.peers.map((peer) => {
    if (peer.id === 'p2p-boot') return peer;
    const address = addressByPeer.get(peer.id);
    const reason = handshakeFailure(state, address);
    return { ...peer, connected: !reason, failedHandshakeReason: reason, knownAddrs: address ? [address] : peer.knownAddrs };
  });
  return { ...state, lastTransition: 'handshake', handshakeCount: state.handshakeCount + state.addressBook.length, peers, messages: state.messages.concat(state.addressBook.map((address) => message(state.tick, 'local-node', address.peerId, '握手', handshakeFailure(state, address) ? 'dropped' : 'delivered', '握手校验网络标识、协议版本和黑名单状态。'))) };
}

/**
 * probePeers 对已连接节点执行健康探测。
 */
function probePeers(state: DiscoveryState): DiscoveryState {
  return { ...state, lastTransition: 'probe', peers: state.peers.map((peer) => ({ ...peer, healthy: peer.connected && !peer.malicious && !peer.failedHandshakeReason })), messages: state.messages.concat(state.peers.filter((peer) => peer.connected).map((peer) => message(state.tick, 'local-node', peer.id, 'ping/pong', peer.malicious ? 'dropped' : 'delivered', '健康探测确认连接是否继续保留。'))) };
}

/**
 * poisonAddressBook 注入错误网络和高分地址,模拟地址投毒。
 */
function poisonAddressBook(state: DiscoveryState): DiscoveryState {
  const poisoned = state.peers.map((peer) => (peer.id === 'p2p-d' ? { ...peer, malicious: true, networkId: 'evil-net', protocolVersion: 99 } : peer));
  const poisonAddress: DiscoveryAddress = { peerId: 'p2p-d', networkId: 'evil-net', protocolVersion: 99, score: 95, source: 'unknown' };
  return { ...state, lastTransition: 'poison', peers: poisoned, addressBook: dedupeAddresses(state.addressBook.concat(poisonAddress)) };
}

/**
 * banInvalidPeers 剔除恶意、错误网络或不兼容版本节点。
 */
function banInvalidPeers(state: DiscoveryState): DiscoveryState {
  const bannedPeerIds = Array.from(new Set(state.peers.filter((peer) => peer.malicious || peer.failedHandshakeReason).map((peer) => peer.id)));
  return { ...state, lastTransition: 'ban', bannedPeerIds, peers: state.peers.map((peer) => (bannedPeerIds.includes(peer.id) ? { ...peer, connected: false, healthy: false, banned: true } : peer)), addressBook: state.addressBook.filter((address) => !bannedPeerIds.includes(address.peerId)) };
}

/**
 * handshakeFailure 返回握手失败原因,空值表示通过。
 */
function handshakeFailure(state: DiscoveryState, address?: DiscoveryAddress): string | undefined {
  if (!address) return '无地址';
  if (state.bannedPeerIds.includes(address.peerId)) return '已在黑名单';
  if (address.networkId !== state.localNetworkId) return '网络不匹配';
  if (address.protocolVersion < state.minProtocolVersion) return '版本过低';
  return undefined;
}

/**
 * dedupeAddresses 保留每个 peer 的最高分地址。
 */
function dedupeAddresses(addresses: DiscoveryAddress[]): DiscoveryAddress[] {
  const byPeer = new Map<string, DiscoveryAddress>();
  for (const address of addresses) {
    const current = byPeer.get(address.peerId);
    if (!current || address.score > current.score) byPeer.set(address.peerId, address);
  }
  return Array.from(byPeer.values()).sort((left, right) => right.score - left.score);
}

/**
 * broadcast 创建地址交换消息。
 */
function broadcast(state: DiscoveryState, from: string, label: string, detail: string): NetworkMessageView[] {
  return state.peers.filter((peer) => peer.id !== from).map((peer) => message(state.tick, from, peer.id, label, peer.malicious ? 'dropped' : 'delivered', detail));
}

/**
 * message 创建带过程信息的节点发现消息。
 */
function message(at: number, from: string, to: string, label: string, status: NetworkMessageView['status'], detail: string): NetworkMessageView {
  return processNetworkMessage(at, { id: deterministicId('p2p-msg', { from, to, label, at, status }), from, to, at, label, status }, detail);
}

/**
 * explainDiscoveryPhase 生成阶段说明。
 */
function explainDiscoveryPhase(index: number) {
  const phase = discoveryPhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
