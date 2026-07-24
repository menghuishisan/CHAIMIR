// 本文件提供共识仿真共享的可视化数据构造函数,不包含任何具体共识算法状态机。

import type { ChartSeries, GraphNode, MatrixCell } from '../../types';
import { timedVisualMessage } from '../packageTools';
export { labeledLaneMessages as laneMessages, matrixCells, messageGraphEdges as graphEdges, pipelineSteps } from '../packageTools';

export interface ViewNode {
  id: string;
  label: string;
  role: string;
  status: 'idle' | 'active' | 'success' | 'warning' | 'danger';
  value?: string;
}

export interface ViewMessage {
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
 * graphNodes 把共识节点语义直接交给图渲染器。
 */
export function graphNodes(nodes: ViewNode[]): GraphNode[] {
  return nodes;
}

/**
 * pipelineSteps 根据阶段序号生成流水线运行状态。
 */
/**
 * processViewMessage 为共识消息附加发送到到达的过程片段。
 */
export function processViewMessage(currentTick: number, message: ViewMessage, detail: string): ViewMessage {
  return timedVisualMessage(currentTick, message, detail);
}

/**
 * refreshViewMessages 按当前 tick 统一刷新历史消息进度。
 */
export function refreshViewMessages(messages: ViewMessage[], currentTick: number, detailOf: (message: ViewMessage) => string): ViewMessage[] {
  return messages.map((message) => processViewMessage(currentTick, message, detailOf(message)));
}

/**
 * voteCells 统一生成共识类投票矩阵。
 */
export function voteCells(rows: string[], columns: string[], voted: (row: string, column: string) => MatrixCell): MatrixCell[][] {
  return rows.map((row) => columns.map((column) => voted(row, column)));
}

/**
 * trendSeries 输出共识仿真常用的法定条件、风险和最终性趋势。
 */
export function trendSeries(points: Array<{ x: number; quorum: number; risk: number; finality: number }>): ChartSeries[] {
  return [
    { label: '法定条件', points: points.map((point) => ({ x: point.x, y: point.quorum })) },
    { label: '风险', points: points.map((point) => ({ x: point.x, y: point.risk })) },
    { label: '最终性', points: points.map((point) => ({ x: point.x, y: point.finality })) },
  ];
}
