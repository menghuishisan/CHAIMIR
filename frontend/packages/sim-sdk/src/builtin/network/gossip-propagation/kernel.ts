// 本文件实现 Gossip 种子、扇出、逐轮传播、去重、覆盖收敛和污染隔离内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { networkMessageId, pollutedMessageId } from '../networkPrimitives';
import { processNetworkMessage, refreshNetworkMessages, type NetworkMessageView } from '../networkView';
import { gossipPhases, type GossipPeer, type GossipState } from './model';
import { traceLinesForGossip } from './trace';

/**
 * createInitialGossipState 创建六节点 Gossip 网络。
 */
export function createInitialGossipState(_params: SimInitParams, _seed: number): GossipState {
  const ids = ['A', 'B', 'C', 'D', 'E', 'F'].map((label) => `gossip-${label.toLowerCase()}`);
  const peers = ids.map<GossipPeer>((id, index) => ({ id, label: `节点 ${String.fromCharCode(65 + index)}`, role: 'gossip-peer', status: index === 0 ? 'active' : 'idle', value: index === 0 ? '种子' : '等待', informed: index === 0, duplicateCount: 0, polluted: false, seenMessageIds: index === 0 ? ['msg-main'] : [], neighbors: [ids[(index + 1) % ids.length], ids[(index + 2) % ids.length], ids[(index + ids.length - 1) % ids.length]], activeSender: index === 0 }));
  return finalizeGossipState({ tick: 0, phase: gossipPhases[0].label, phaseIndex: 0, fanout: 2, messageId: 'msg-main', round: 0, frontier: [ids[0]], peers, messages: [], samples: [{ x: 0, coverage: 17, risk: 8, latency: 5 }], lastTransition: 'seed', explanation: explainGossipPhase(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceGossipEvent 是 Gossip 包唯一事件入口。
 */
export function reduceGossipEvent(state: GossipState, event: SimEvent, _context: ReducerContext): GossipState {
  if (event.type === 'select') return finalizeGossipState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeGossipState(polluteGossip(state));
  if (event.type === 'recover') return finalizeGossipState(quarantinePollution(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeGossipState(advanceGossip(state, event));
  return state;
}

/**
 * advanceGossip 按 Gossip 传播流程推进一个过程单元。
 */
export function advanceGossip(state: GossipState, event: SimEvent): GossipState {
  const phaseIndex = Math.min(gossipPhases.length - 1, state.phaseIndex + 1);
  const base = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: gossipPhases[phaseIndex].id };
  if (phaseIndex === 1 || phaseIndex === 2) return spreadRound(base);
  if (phaseIndex === 3) return dedupeRound(base);
  if (phaseIndex === 4) return converge(base);
  return base;
}

/**
 * coverageCheckpoint 输出覆盖率检查点。
 */
export function coverageCheckpoint(state: GossipState): CheckpointResult {
  const covered = coverage(state);
  return { achieved: covered >= 80 && !state.peers.some((peer) => peer.polluted), answer: { coverage: covered, fanout: state.fanout }, explanation: covered >= 80 ? 'Gossip 已覆盖大多数节点。' : '覆盖率不足,需要继续传播或调整扇出。' };
}

/**
 * finalizeGossipState 刷新指标、趋势、消息进度和代码追踪。
 */
export function finalizeGossipState(state: GossipState): GossipState {
  const covered = coverage(state);
  const risk = state.peers.some((peer) => peer.polluted) ? 78 : covered >= 80 ? 10 : 28;
  const samples = state.samples.concat({ x: state.tick + state.phaseIndex, coverage: covered, risk, latency: Math.min(100, state.messages.length * 2) }).slice(-24);
  return {
    ...state,
    phase: gossipPhases[state.phaseIndex].label,
    peers: state.peers.map((peer) => ({ ...peer, status: peer.polluted ? 'danger' : peer.activeSender ? 'active' : peer.informed ? 'success' : 'idle', value: peer.informed ? '已收到' : '等待' })),
    messages: refreshNetworkMessages(state.messages, state.tick, (message) => message.detail ?? 'Gossip 消息正在传播。'),
    samples,
    explanation: explainGossipPhase(state.phaseIndex),
    metrics: { result: covered >= 80 ? '传播收敛' : '继续扩散', coverage: covered, risk, round: state.round },
    checkpointValues: { coverage: covered >= 80 && !state.peers.some((peer) => peer.polluted) },
    _trace: { triggeredLines: traceLinesForGossip(state.lastTransition), variables: { coverage: covered, fanout: state.fanout, round: state.round }, executionPath: `gossip/${state.lastTransition}` },
  };
}

/**
 * spreadRound 让当前 frontier 按 fanout 向邻居扩散。
 */
function spreadRound(state: GossipState): GossipState {
  const nextPeers = state.peers.map((peer) => ({ ...peer, activeSender: false, seenMessageIds: [...peer.seenMessageIds] }));
  const nextFrontier = new Set<string>();
  const messages: NetworkMessageView[] = [];
  for (const sourceId of state.frontier) {
    const source = state.peers.find((peer) => peer.id === sourceId);
    if (!source || source.polluted) continue;
    for (const targetId of source.neighbors.slice(0, state.fanout)) {
      const target = nextPeers.find((peer) => peer.id === targetId);
      if (!target) continue;
      messages.push(message(state.tick, source.id, target.id, '传播消息', target.polluted ? 'dropped' : 'delivered', 'Gossip 节点向 fanout 邻居传播消息。'));
      if (target.seenMessageIds.includes(state.messageId)) {
        target.duplicateCount += 1;
        continue;
      }
      target.seenMessageIds.push(state.messageId);
      target.informed = true;
      target.activeSender = true;
      nextFrontier.add(target.id);
    }
  }
  return { ...state, lastTransition: state.phaseIndex === 1 ? 'fanout' : 'spread', round: state.round + 1, frontier: Array.from(nextFrontier), peers: nextPeers, messages: state.messages.concat(messages).slice(-36) };
}

/**
 * dedupeRound 对重复消息计数但不再进入下一轮 frontier。
 */
function dedupeRound(state: GossipState): GossipState {
  return { ...state, lastTransition: 'dedupe', peers: state.peers.map((peer) => ({ ...peer, activeSender: state.frontier.includes(peer.id), duplicateCount: peer.duplicateCount + (peer.informed && !state.frontier.includes(peer.id) ? 1 : 0) })) };
}

/**
 * converge 进入覆盖收敛阶段。
 */
function converge(state: GossipState): GossipState {
  return { ...state, lastTransition: 'converge', frontier: [] };
}

/**
 * polluteGossip 注入污染消息源。
 */
function polluteGossip(state: GossipState): GossipState {
  return { ...state, lastTransition: 'pollute', peers: state.peers.map((peer) => (peer.id === 'gossip-c' ? { ...peer, polluted: true, informed: true, seenMessageIds: peer.seenMessageIds.concat(pollutedMessageId(state.tick)) } : peer)) };
}

/**
 * quarantinePollution 隔离污染节点并保留其他节点的已知状态。
 */
function quarantinePollution(state: GossipState): GossipState {
  return { ...state, lastTransition: 'dedupe', peers: state.peers.map((peer) => (peer.polluted ? { ...peer, polluted: false, informed: false, activeSender: false, duplicateCount: 0, seenMessageIds: [] } : peer)) };
}

/**
 * coverage 计算已收到消息节点比例。
 */
export function coverage(state: GossipState): number {
  return Math.round((state.peers.filter((peer) => peer.informed && !peer.polluted).length / state.peers.length) * 100);
}

/**
 * message 创建 Gossip 消息。
 */
function message(at: number, from: string, to: string, label: string, status: NetworkMessageView['status'], detail: string): NetworkMessageView {
  return processNetworkMessage(at, { id: networkMessageId('gossip-msg', { from, to, label, at, status }), from, to, label, at, status }, detail);
}

/**
 * explainGossipPhase 生成阶段说明。
 */
function explainGossipPhase(index: number) {
  const phase = gossipPhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
