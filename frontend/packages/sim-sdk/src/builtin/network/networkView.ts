// 本文件提供网络传播仿真共享的展示数据辅助,只做拓扑、消息和矩阵语义转换。

import type { ChartSeries, GraphEdge, GraphNode, LaneMessage, MatrixCell, PipelineStep } from '../../types';

export interface NetworkNodeView {
  id: string;
  label: string;
  role: string;
  status: 'idle' | 'active' | 'success' | 'warning' | 'danger';
  value?: string;
}

export interface NetworkMessageView {
  id: string;
  from: string;
  to: string;
  at: number;
  label: string;
  status: 'sent' | 'delivered' | 'dropped';
  endAt?: number;
  detail?: string;
  process?: {
    startedAt: number;
    endedAt: number;
    progress: number;
    label: string;
  };
}

/**
 * graphNodes 将网络节点转换为图节点。
 */
export function graphNodes(nodes: NetworkNodeView[]): GraphNode[] {
  return nodes;
}

/**
 * graphEdges 将网络消息转换为图边。
 */
export function graphEdges(messages: NetworkMessageView[]): GraphEdge[] {
  return messages.map((message) => ({ id: message.id, from: message.from, to: message.to, label: message.label, status: message.status === 'dropped' ? 'failed' : message.status === 'delivered' ? 'success' : 'active', process: message.process, detail: message.detail }));
}

/**
 * laneMessages 把内部节点 ID 替换为展示名称。
 */
export function laneMessages(messages: NetworkMessageView[], labelOf: (id: string) => string): LaneMessage[] {
  return messages.map((message) => ({ ...message, from: labelOf(message.from), to: labelOf(message.to) }));
}

/**
 * pipelineSteps 生成网络机制阶段流水线。
 */
export function pipelineSteps(phases: Array<{ id: string; label: string; detail: string }>, activeIndex: number, failed = false): PipelineStep[] {
  return phases.map((phase, index) => ({ id: phase.id, label: phase.label, detail: phase.detail, status: index < activeIndex ? 'complete' : index === activeIndex ? (failed ? 'failed' : 'running') : 'pending' }));
}

/**
 * processNetworkMessage 为网络消息附加发送到到达的过程片段。
 */
export function processNetworkMessage(currentTick: number, message: NetworkMessageView, detail: string): NetworkMessageView {
  const endAt = message.endAt ?? message.at + 1;
  const progress = Math.min(1, Math.max(0, (currentTick - message.at) / Math.max(1, endAt - message.at)));
  return { ...message, endAt, detail, process: { startedAt: message.at, endedAt: endAt, progress, label: message.label } };
}

/**
 * refreshNetworkMessages 按当前 tick 刷新历史网络消息进度。
 */
export function refreshNetworkMessages(messages: NetworkMessageView[], currentTick: number, detailOf: (message: NetworkMessageView) => string): NetworkMessageView[] {
  return messages.map((message) => processNetworkMessage(currentTick, message, detailOf(message)));
}

/**
 * matrixCells 生成网络状态矩阵。
 */
export function matrixCells(rows: string[], columns: string[], read: (row: string, column: string) => MatrixCell): MatrixCell[][] {
  return rows.map((row) => columns.map((column) => read(row, column)));
}

/**
 * metricSeries 生成覆盖率、风险和延迟趋势。
 */
export function metricSeries(points: Array<{ x: number; coverage: number; risk: number; latency: number }>): ChartSeries[] {
  return [
    { label: '覆盖率', points: points.map((point) => ({ x: point.x, y: point.coverage })) },
    { label: '风险', points: points.map((point) => ({ x: point.x, y: point.risk })) },
    { label: '延迟', points: points.map((point) => ({ x: point.x, y: point.latency })) },
  ];
}
