// 本文件把 PBFT 协议内核状态映射为封闭可视化模式,不改变协议状态。

import type { GraphEdge, GraphNode, LaneMessage, MatrixCell, TeachingFrame } from '../../../types';
import { teachingFrame, graphPattern, lanePattern, matrixPattern, selectedOrFrameFocus } from '../../packageTools';
import { quorum } from './kernel';
import type { PbftMessage, PbftReplica, PbftState } from './model';

/**
 * renderPbftView 输出 PBFT 的网络图、过程流水线、时序泳道和证书矩阵。
 */
export function renderPbftView(state: PbftState): TeachingFrame {
  const preparedCount = state.replicas.filter((replica) => replica.preparedDigest === state.request.digest).length;
  const committedCount = state.replicas.filter((replica) => replica.committedDigest === state.request.digest).length;
  const threshold = quorum(state);
  const summary = `视图 ${state.view},序号 ${state.sequence},准备证书 ${preparedCount}/${threshold},提交证书 ${committedCount}/${threshold},当前过程 ${state.phase},风险 ${state.metrics.risk}。`;
  const patterns = [
    graphPattern('pbft-graph', 'PBFT 主节点广播与副本投票网络', pbftNodes(state), pbftEdges(state)),
    lanePattern('pbft-lane', 'PBFT pre-prepare / prepare / commit 时序', actorLabels(state), pbftLaneMessages(state), state.tick),
    matrixPattern('pbft-matrix', `2f+1 证书矩阵,还差准备 ${Math.max(0, threshold - preparedCount)},提交 ${Math.max(0, threshold - committedCount)}`, state.replicas.map((replica) => replica.label), ['预准备', '准备证书', '提交证书', '客户端回复', '稳定检查点'], pbftCells(state)),
  ];
  return teachingFrame({
    summary,
    phase: {
      id: state.phase,
      title: state.explanation.title,
      intent: 'observe',
      what: state.explanation.effect,
      why: state.explanation.reason,
      watch: summary,
    },
    focus: {
      primary: selectedOrFrameFocus(state.selectedElementId, ['pbft-graph']),
      secondary: ['pbft-lane', 'pbft-matrix'],
    },
    layout: {
      primary: 'pbft-graph',
      evidence: ['pbft-matrix'],
      timeline: 'pbft-lane',
    },
    patterns,
  });
}

/**
 * pbftNodes 将副本协议状态映射为图节点。
 */
function pbftNodes(state: PbftState): GraphNode[] {
  return state.replicas.map((replica) => ({
    id: replica.id,
    label: replica.label,
    role: 'participant',
    status: nodeStatus(replica),
    value: replica.primary ? '主节点' : replica.stableCheckpoint ? '检查点' : replica.committedDigest ? '提交' : replica.preparedDigest ? '准备' : '副本',
  }));
}

/**
 * pbftEdges 将消息起止 tick 转换为连线上的过程进度。
 */
function pbftEdges(state: PbftState): GraphEdge[] {
  return state.messages
    .filter((message) => message.to !== 'all' && message.from !== state.request.clientId && message.to !== state.request.clientId)
    .map((message) => ({
      id: message.id,
      from: message.from,
      to: message.to,
      label: message.type,
      status: message.accepted ? (state.tick >= message.endTick ? 'success' : 'active') : 'failed',
      detail: message.detail,
      process: processSpan(state, message),
    }));
}

/**
 * pbftLaneMessages 将协议消息映射到多参与方时序泳道,同一消息在发送方和接收方处连续移动。
 */
function pbftLaneMessages(state: PbftState): LaneMessage[] {
  return state.messages
    .filter((message) => message.to !== 'all')
    .map((message) => ({
      id: message.id,
      from: labelOf(state, message.from),
      to: labelOf(state, message.to),
      at: message.startTick,
      endAt: message.endTick,
      label: message.type,
      status: message.accepted ? (state.tick >= message.endTick ? 'delivered' : 'sent') : 'dropped',
      detail: message.detail,
      process: processSpan(state, message),
    }));
}

/**
 * pbftCells 展示每个副本是否接受预准备、形成准备、提交、回复和检查点。
 */
function pbftCells(state: PbftState): MatrixCell[][] {
  return state.replicas.map((replica) => [
    cell(Boolean(replica.acceptedPrePrepare), replica.faulty ? '异常' : replica.acceptedPrePrepare ? '接受' : '等待', replica.faulty),
    cell(replica.preparedDigest === state.request.digest, replica.preparedDigest ? '通过' : '等待'),
    cell(replica.committedDigest === state.request.digest, replica.committedDigest ? '提交' : '等待'),
    cell(Boolean(replica.repliedDigest), replica.repliedDigest ? '已回复' : '等待'),
    cell(replica.stableCheckpoint === state.sequence, replica.stableCheckpoint ? '稳定' : '等待'),
  ]);
}

/**
 * processSpan 将内核消息的起止 tick 转换为渲染协议的过程片段。
 */
function processSpan(state: PbftState, message: PbftMessage) {
  const duration = Math.max(1, message.endTick - message.startTick);
  return {
    startedAt: message.startTick,
    endedAt: message.endTick,
    progress: Math.min(1, Math.max(0, (state.tick - message.startTick) / duration)),
    label: message.detail,
  };
}

/**
 * cell 构造矩阵单元,避免颜色成为唯一状态表达。
 */
function cell(ok: boolean, label: string, fault = false): MatrixCell {
  if (fault) return { label, status: 'fault' };
  return { label, status: ok ? 'yes' : 'empty' };
}

/**
 * nodeStatus 返回副本在当前协议下的视觉状态。
 */
function nodeStatus(replica: PbftReplica): GraphNode['status'] {
  if (replica.faulty) return 'danger';
  if (replica.primary) return 'active';
  if (replica.stableCheckpoint || replica.repliedDigest) return 'success';
  if (replica.preparedDigest || replica.committedDigest) return 'warning';
  return 'idle';
}

/**
 * actorLabels 返回泳道参与方名称。
 */
function actorLabels(state: PbftState): string[] {
  return ['客户端'].concat(state.replicas.map((replica) => replica.label));
}

/**
 * labelOf 将内部协议 ID 转换为学习者可读标签。
 */
function labelOf(state: PbftState, id: string): string {
  if (id === state.request.clientId) return '客户端';
  return state.replicas.find((replica) => replica.id === id)?.label ?? id;
}
