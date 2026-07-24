// 本文件提供网络传播仿真共享的展示数据辅助,只做拓扑、消息和矩阵语义转换。

import type { ChartSeries, GraphNode } from '../../types';
import { timedVisualMessage } from '../packageTools';

export { labeledLaneMessages as laneMessages, matrixCells, messageGraphEdges as graphEdges, pipelineSteps } from '../packageTools';

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
 * pipelineSteps 生成网络机制阶段流水线。
 */
/**
 * processNetworkMessage 为网络消息附加发送到到达的过程片段。
 */
export function processNetworkMessage(currentTick: number, message: NetworkMessageView, detail: string): NetworkMessageView {
  return timedVisualMessage(currentTick, message, detail);
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
