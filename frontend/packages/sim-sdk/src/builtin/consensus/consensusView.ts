// 本文件提供共识仿真共享的可视化数据构造函数,不包含任何具体共识算法状态机。

import type { ChartSeries, GraphEdge, GraphNode, LaneMessage, MatrixCell, PipelineStep } from '../../types';

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
 * graphEdges 将协议消息映射为图边状态,失败消息以 failed 呈现。
 */
export function graphEdges(messages: ViewMessage[]): GraphEdge[] {
  return messages.map((message) => ({
    id: message.id,
    from: message.from,
    to: message.to,
    label: message.label,
    status: message.status === 'dropped' ? 'failed' : message.status === 'delivered' ? 'success' : 'active',
    detail: message.detail,
    process: message.process,
  }));
}

/**
 * laneMessages 将内部 ID 替换为用户可读泳道名称。
 */
export function laneMessages(messages: ViewMessage[], labelOf: (id: string) => string): LaneMessage[] {
  return messages.map((message) => ({ ...message, from: labelOf(message.from), to: labelOf(message.to) }));
}

/**
 * pipelineSteps 根据阶段序号生成流水线运行状态。
 */
export function pipelineSteps(phases: ReadonlyArray<{ id: string; label: string; detail: string }>, activeIndex: number, failed = false): PipelineStep[] {
  return phases.map((phase, index) => ({
    id: phase.id,
    label: phase.label,
    detail: phase.detail,
    status: index < activeIndex ? 'complete' : index === activeIndex ? (failed ? 'failed' : 'running') : 'pending',
  }));
}

/**
 * processPipelineSteps 为共识过程流水线附加连续进度,避免各算法重复实现进度条逻辑。
 */
export function processPipelineSteps(phases: ReadonlyArray<{ id: string; label: string; detail: string }>, activeIndex: number, currentTick: number, failed = false): PipelineStep[] {
  return pipelineSteps(phases, activeIndex, failed).map((step, index) => ({
    ...step,
    process: {
      startedAt: Math.max(0, currentTick - 1),
      endedAt: currentTick + 1,
      progress: index < activeIndex ? 1 : index === activeIndex ? 0.65 : 0,
      label: step.detail,
    },
  }));
}

/**
 * processViewMessage 为共识消息附加发送到到达的过程片段。
 */
export function processViewMessage(currentTick: number, message: ViewMessage, detail: string): ViewMessage {
  const endAt = message.endAt ?? message.at + 1;
  const progress = Math.min(1, Math.max(0, (currentTick - message.at) / Math.max(1, endAt - message.at)));
  return { ...message, endAt, detail, process: { startedAt: message.at, endedAt: endAt, progress, label: message.label } };
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
