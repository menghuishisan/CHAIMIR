// 本文件实现网络分区拓扑切割、分区内同步、链路恢复和状态合并内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { integerParam } from '../../initParams';
import { processNetworkMessage, refreshNetworkMessages, type NetworkMessageView } from '../networkView';
import { partitionPhases, type PartitionLink, type PartitionNode, type PartitionState } from './model';
import { traceLinesForPartition } from './trace';

/**
 * createInitialPartitionState 根据参数创建左右两区拓扑和跨区割边。
 */
export function createInitialPartitionState(params: SimInitParams, _seed: number): PartitionState {
  const nodeCount = integerParam(params, 'nodeCount', 6, 4, 12);
  const leftCount = integerParam(params, 'leftCount', Math.ceil(nodeCount / 2), 2, nodeCount - 2);
  const initialVersion = integerParam(params, 'initialVersion', 1, 1, 1000);
  const nodes = Array.from({ length: nodeCount }, (_, index): PartitionNode => ({ id: `part-${String.fromCharCode(97 + index)}`, label: `节点 ${String.fromCharCode(65 + index)}`, role: 'partition-node', status: 'idle', value: index < leftCount ? '左区' : '右区', group: index < leftCount ? 'left' : 'right', reachable: true, syncedVersion: initialVersion, localWrites: 0 }));
  const links = createPartitionLinks(nodes, leftCount);
  return finalizePartitionState({ tick: 0, phase: partitionPhases[0].label, phaseIndex: 0, partitionActive: false, nodes, links, messages: [], samples: [{ x: 0, coverage: 100, risk: 8, latency: 12 }], lastTransition: 'topology', explanation: explainPartitionPhase(0), metrics: {}, checkpointValues: {} });
}

/**
 * reducePartitionEvent 是网络分区包唯一事件入口。
 */
export function reducePartitionEvent(state: PartitionState, event: SimEvent, _context: ReducerContext): PartitionState {
  if (event.type === 'select') return finalizePartitionState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizePartitionState(cutPartition(state));
  if (event.type === 'recover') return finalizePartitionState(healPartition(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizePartitionState(advancePartition(state, event));
  return state;
}

/**
 * advancePartition 推进分区处理流程。
 */
export function advancePartition(state: PartitionState, event: SimEvent): PartitionState {
  const phaseIndex = Math.min(partitionPhases.length - 1, state.phaseIndex + 1);
  const next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: partitionPhases[phaseIndex].id };
  if (phaseIndex === 1) return cutPartition(next);
  if (phaseIndex === 2) return localSync(next);
  if (phaseIndex === 3) return healPartition(next);
  if (phaseIndex === 4) return mergePartition(next);
  return next;
}

/**
 * partitionMerged 输出分区恢复检查点。
 */
export function partitionMerged(state: PartitionState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.merged);
  return { achieved, answer: { versionGap: state.metrics.versionGap, partitionActive: state.partitionActive }, explanation: achieved ? '网络已恢复且所有节点版本一致。' : '仍存在网络分区或版本分歧。' };
}

/**
 * finalizePartitionState 刷新分区指标、消息进度和追踪。
 */
export function finalizePartitionState(state: PartitionState): PartitionState {
  const gap = versionGap(state);
  const risk = state.partitionActive ? 86 : gap > 0 ? 44 : 8;
  const reachable = state.nodes.filter((node) => node.reachable).length;
  const samples = state.samples.concat({ x: state.tick + state.phaseIndex, coverage: Math.round((reachable / state.nodes.length) * 100), risk, latency: state.partitionActive ? 90 : 18 }).slice(-24);
  return {
    ...state,
    phase: partitionPhases[state.phaseIndex].label,
    nodes: state.nodes.map((node) => ({ ...node, status: !node.reachable ? 'danger' : gap === 0 ? 'success' : 'warning' })),
    messages: refreshNetworkMessages(state.messages, state.tick, (message) => message.detail ?? '分区恢复消息正在传播。'),
    samples,
    explanation: explainPartitionPhase(state.phaseIndex),
    metrics: { result: gap === 0 && !state.partitionActive ? '状态已合并' : '存在分歧', risk, versionGap: gap },
    checkpointValues: { merged: gap === 0 && !state.partitionActive },
    _trace: { triggeredLines: traceLinesForPartition(state.lastTransition), variables: { partitionActive: state.partitionActive, versionGap: gap }, executionPath: `partition/${state.lastTransition}` },
  };
}

/**
 * cutPartition 切断跨区链路。
 */
function cutPartition(state: PartitionState): PartitionState {
  return { ...state, lastTransition: 'cut', partitionActive: true, links: state.links.map((link) => (link.crossRegion ? { ...link, cut: true } : link)), nodes: state.nodes.map((node) => ({ ...node, reachable: node.group === 'left' })), messages: state.messages.concat(crossMessages(state, '跨区消息阻断', true)) };
}

/**
 * localSync 让两个分区产生不同版本和本地写入。
 */
function localSync(state: PartitionState): PartitionState {
  return { ...state, lastTransition: 'local-sync', nodes: state.nodes.map((node) => ({ ...node, localWrites: node.group === 'left' ? 1 : 2, syncedVersion: node.group === 'left' ? 2 : 3 })) };
}

/**
 * healPartition 恢复跨区链路并交换版本。
 */
function healPartition(state: PartitionState): PartitionState {
  return { ...state, lastTransition: 'heal', partitionActive: false, links: state.links.map((link) => ({ ...link, cut: false })), nodes: state.nodes.map((node) => ({ ...node, reachable: true })), messages: state.messages.concat(crossMessages(state, '恢复同步', false)) };
}

/**
 * mergePartition 按较高版本合并状态。
 */
function mergePartition(state: PartitionState): PartitionState {
  const targetVersion = Math.max(...state.nodes.map((node) => node.syncedVersion));
  return { ...state, lastTransition: 'merge', nodes: state.nodes.map((node) => ({ ...node, syncedVersion: targetVersion })) };
}

/**
 * createPartitionLinks 构造线性拓扑,左右分区交界处唯一链路标记为割边。
 */
function createPartitionLinks(nodes: PartitionNode[], leftCount: number): PartitionLink[] {
  const links: PartitionLink[] = [];
  for (let index = 0; index < nodes.length - 1; index += 1) {
    links.push({ id: `link-${nodes[index].id}-${nodes[index + 1].id}`, from: nodes[index].id, to: nodes[index + 1].id, crossRegion: index === leftCount - 1, cut: false });
  }
  return links;
}

/**
 * versionGap 计算最大最小版本差。
 */
export function versionGap(state: PartitionState): number {
  const versions = state.nodes.map((node) => node.syncedVersion);
  return Math.max(...versions) - Math.min(...versions);
}

/**
 * crossMessages 创建跨区消息。
 */
function crossMessages(state: PartitionState, label: string, dropped: boolean): NetworkMessageView[] {
  const link = state.links.find((item) => item.crossRegion) ?? state.links[0];
  return [processNetworkMessage(state.tick, { id: deterministicId('partition-msg', { label, tick: state.tick, dropped }), from: link.from, to: link.to, at: state.tick, label, status: dropped ? 'dropped' : 'delivered' }, dropped ? '跨区割边阻断消息。' : '恢复链路后交换版本信息。')];
}

/**
 * explainPartitionPhase 生成阶段说明。
 */
function explainPartitionPhase(index: number) {
  const phase = partitionPhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
