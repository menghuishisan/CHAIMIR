// 本文件实现 DHT ID 空间、K 桶、异或距离、迭代查询和污染路由修复内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { networkMessageId } from '../networkPrimitives';
import { processNetworkMessage, refreshNetworkMessages, type NetworkMessageView } from '../networkView';
import { dhtPhases, type DhtPeer, type DhtState } from './model';
import { traceLinesForDht } from './trace';

/**
 * createInitialDhtState 创建 DHT 节点、目标 key 和初始短名单。
 */
export function createInitialDhtState(_params: SimInitParams, _seed: number): DhtState {
  const lookupKey = 173;
  const nodeIds = [12, 45, 91, 160, 176, 225];
  const peers = nodeIds.map<DhtPeer>((nodeId, index) => ({ id: `dht-${index + 1}`, label: `节点 ${index + 1}`, role: 'dht-peer', status: 'idle', value: `ID ${nodeId}`, nodeId, bucket: bucketFor(nodeId, lookupKey), queried: index === 0, inShortlist: index < 3, closest: false, polluted: false, hasValue: index === 4, returnedPeers: nodeIds.filter((id) => id !== nodeId).slice(index % 3, index % 3 + 3) }));
  return finalizeDhtState({ tick: 0, phase: dhtPhases[0].label, phaseIndex: 0, lookupKey, alpha: 2, peers, shortlist: nearest(peers, lookupKey, 3).map((peer) => peer.id), messages: [], hops: 0, foundValue: false, lastTransition: 'id-space', explanation: explainDhtPhase(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceDhtEvent 是 DHT 路由包唯一事件入口。
 */
export function reduceDhtEvent(state: DhtState, event: SimEvent, _context: ReducerContext): DhtState {
  if (event.type === 'select') return finalizeDhtState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeDhtState(polluteRoute(state));
  if (event.type === 'recover') return finalizeDhtState(repairRoute(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeDhtState(advanceDht(state, event));
  return state;
}

/**
 * advanceDht 按 DHT 查找流程推进一个过程单元。
 */
export function advanceDht(state: DhtState, event: SimEvent): DhtState {
  const phaseIndex = Math.min(dhtPhases.length - 1, state.phaseIndex + 1);
  const next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: dhtPhases[phaseIndex].id };
  if (phaseIndex === 1) return refreshBuckets(next);
  if (phaseIndex === 2) return refreshShortlist(next, 'distance');
  if (phaseIndex === 3) return queryClosest(next);
  if (phaseIndex === 4) return repairRoute(next);
  return next;
}

/**
 * lookupFound 输出 DHT 查找检查点。
 */
export function lookupFound(state: DhtState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.found);
  return { achieved, answer: { hops: state.hops, key: state.lookupKey, shortlist: state.shortlist }, explanation: achieved ? '查询沿异或距离找到目标值。' : '尚未找到目标或路由被污染。' };
}

/**
 * finalizeDhtState 刷新 DHT 指标、消息进度和代码追踪。
 */
export function finalizeDhtState(state: DhtState): DhtState {
  const polluted = state.shortlist.some((id) => state.peers.find((peer) => peer.id === id)?.polluted === true);
  const shortlist = new Set(state.shortlist);
  return {
    ...state,
    phase: dhtPhases[state.phaseIndex].label,
    peers: state.peers.map((peer) => ({ ...peer, inShortlist: shortlist.has(peer.id), status: peer.polluted ? 'danger' : peer.closest ? 'active' : shortlist.has(peer.id) ? 'warning' : peer.queried ? 'success' : 'idle' })),
    messages: refreshNetworkMessages(state.messages, state.tick, (message) => message.detail ?? 'DHT 查询消息正在传播。'),
    explanation: explainDhtPhase(state.phaseIndex),
    metrics: { result: state.foundValue && !polluted ? '已找到值' : '继续查找', risk: polluted ? 78 : 15, hops: state.hops, shortlistSize: state.shortlist.length },
    checkpointValues: { found: state.foundValue && !polluted },
    _trace: { triggeredLines: traceLinesForDht(state.lastTransition), variables: { lookupKey: state.lookupKey, hops: state.hops, shortlistSize: state.shortlist.length }, executionPath: `dht/${state.lastTransition}` },
  };
}

/**
 * refreshBuckets 依据目标 key 更新每个节点所在 K 桶。
 */
function refreshBuckets(state: DhtState): DhtState {
  return { ...state, lastTransition: 'bucket', peers: state.peers.map((peer) => ({ ...peer, bucket: bucketFor(peer.nodeId, state.lookupKey) })) };
}

/**
 * refreshShortlist 根据异或距离刷新短名单。
 */
function refreshShortlist(state: DhtState, transition: string): DhtState {
  return { ...state, lastTransition: transition, shortlist: nearest(state.peers.filter((peer) => !peer.polluted), state.lookupKey, 4).map((peer) => peer.id) };
}

/**
 * queryClosest 查询 alpha 个最近未查询候选并合并返回节点。
 */
function queryClosest(state: DhtState): DhtState {
  const candidates = state.shortlist
    .map((id) => state.peers.find((peer) => peer.id === id))
    .filter((peer): peer is DhtPeer => peer !== undefined && !peer.queried && !peer.polluted)
    .sort((left, right) => distance(left, state.lookupKey) - distance(right, state.lookupKey))
    .slice(0, state.alpha);
  const returnedIds = candidates.flatMap((peer) => peer.returnedPeers).map((nodeId) => state.peers.find((peer) => peer.nodeId === nodeId)?.id).filter((id): id is string => Boolean(id));
  const shortlist = Array.from(new Set(state.shortlist.concat(returnedIds)));
  const queriedIds = new Set(candidates.map((peer) => peer.id));
  const foundValue = state.foundValue || candidates.some((peer) => peer.hasValue);
  return { ...state, lastTransition: 'query', hops: state.hops + candidates.length, foundValue, shortlist: sortShortlist(state.peers, shortlist, state.lookupKey).slice(0, 4), peers: state.peers.map((peer) => ({ ...peer, queried: peer.queried || queriedIds.has(peer.id), closest: foundValue && peer.hasValue })), messages: state.messages.concat(candidates.map((peer) => message(state.tick, 'local-node', peer.id, 'FIND_VALUE', 'delivered', '查询最近未访问候选并合并返回节点。'))) };
}

/**
 * polluteRoute 注入一个看似更近但返回错误候选的污染节点。
 */
function polluteRoute(state: DhtState): DhtState {
  return { ...state, lastTransition: 'pollute', foundValue: false, shortlist: Array.from(new Set(['dht-4'].concat(state.shortlist))), peers: state.peers.map((peer) => (peer.id === 'dht-4' ? { ...peer, polluted: true, closest: true, returnedPeers: [12, 45] } : peer)) };
}

/**
 * repairRoute 剔除污染候选并重新排序短名单。
 */
function repairRoute(state: DhtState): DhtState {
  const peers = state.peers.map((peer) => (peer.polluted ? { ...peer, closest: false, inShortlist: false } : peer));
  const repaired = refreshShortlist({ ...state, peers, shortlist: state.shortlist.filter((id) => !peers.find((peer) => peer.id === id)?.polluted), foundValue: peers.some((peer) => peer.hasValue && peer.queried) }, 'repair');
  return { ...repaired, foundValue: repaired.foundValue || nearest(peers.filter((peer) => !peer.polluted), state.lookupKey, 1)[0]?.hasValue === true };
}

/**
 * nearest 返回按 XOR 距离排序的前 count 个节点。
 */
function nearest(peers: DhtPeer[], key: number, count: number): DhtPeer[] {
  return [...peers].sort((left, right) => distance(left, key) - distance(right, key)).slice(0, count);
}

/**
 * sortShortlist 对短名单 ID 按 XOR 距离排序。
 */
function sortShortlist(peers: DhtPeer[], ids: string[], key: number): string[] {
  return ids.sort((left, right) => distance(peers.find((peer) => peer.id === left) ?? peers[0], key) - distance(peers.find((peer) => peer.id === right) ?? peers[0], key));
}

/**
 * bucketFor 计算节点相对目标 key 的 K 桶编号。
 */
function bucketFor(nodeId: number, key: number): number {
  return Math.floor(Math.log2(nodeId ^ key || 1));
}

/**
 * distance 计算异或距离。
 */
export function distance(peer: DhtPeer, key: number): number {
  return peer.nodeId ^ key;
}

/**
 * message 创建 DHT 查询消息。
 */
function message(at: number, from: string, to: string, label: string, status: NetworkMessageView['status'], detail: string): NetworkMessageView {
  return processNetworkMessage(at, { id: networkMessageId('dht-msg', { from, to, label, at }), from, to, label, at, status }, detail);
}

/**
 * explainDhtPhase 生成阶段说明。
 */
function explainDhtPhase(index: number) {
  const phase = dhtPhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
