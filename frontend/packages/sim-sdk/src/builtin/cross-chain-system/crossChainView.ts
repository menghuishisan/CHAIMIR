// 本文件提供跨链系统仿真共享展示辅助,只做链、委员会和消息语义转换。

import type { GraphEdge, GraphNode, LaneMessage, MatrixCell, PipelineStep, ProcessSpan } from '../../types';

export interface CrossActor {
  id: string;
  label: string;
  role: string;
  status: 'idle' | 'active' | 'success' | 'warning' | 'danger';
  value?: string;
}

export interface CrossMessage {
  id: string;
  from: string;
  to: string;
  at: number;
  endAt?: number;
  label: string;
  status: 'sent' | 'delivered' | 'dropped';
  detail?: string;
  process?: ProcessSpan;
}

/**
 * graphNodes 将跨链参与方转换为图节点。
 */
export function graphNodes(actors: CrossActor[]): GraphNode[] {
  return actors;
}

/**
 * graphEdges 将跨链消息转换为图边。
 */
export function graphEdges(messages: CrossMessage[]): GraphEdge[] {
  return messages.map((message) => ({ id: message.id, from: message.from, to: message.to, label: message.label, status: message.status === 'dropped' ? 'failed' : message.status === 'delivered' ? 'success' : 'active', process: message.process, detail: message.detail }));
}

/**
 * laneMessages 把内部 ID 转为泳道名称。
 */
export function laneMessages(messages: CrossMessage[], labelOf: (id: string) => string): LaneMessage[] {
  return messages.map((message) => ({ ...message, from: labelOf(message.from), to: labelOf(message.to) }));
}

/**
 * matrixCells 生成跨链检查矩阵。
 */
export function matrixCells(rows: string[], columns: string[], read: (row: string, column: string) => MatrixCell): MatrixCell[][] {
  return rows.map((row) => columns.map((column) => read(row, column)));
}

/**
 * pipelineSteps 生成跨链流程阶段。
 */
export function pipelineSteps(phases: Array<{ id: string; label: string; detail: string }>, activeIndex: number, failed = false): PipelineStep[] {
  return phases.map((phase, index) => ({ id: phase.id, label: phase.label, detail: phase.detail, status: index < activeIndex ? 'complete' : index === activeIndex ? (failed ? 'failed' : 'running') : 'pending', process: processSpan(index, activeIndex, phase.label) }));
}

/**
 * processCrossMessage 为跨链消息附加过程跨度。
 */
export function processCrossMessage(message: Omit<CrossMessage, 'endAt' | 'process' | 'detail'>, detail: string): CrossMessage {
  const endedAt = message.at + 3;
  return { ...message, endAt: endedAt, detail, process: { startedAt: message.at, endedAt, progress: message.status === 'sent' ? 0.45 : message.status === 'dropped' ? 0.75 : 1, label: detail } };
}

/**
 * processSpan 给跨链流程阶段附加过程进度。
 */
function processSpan(index: number, activeIndex: number, label: string): ProcessSpan {
  const startedAt = index * 2;
  const endedAt = startedAt + 2;
  const progress = index < activeIndex ? 1 : index === activeIndex ? 0.6 : 0;
  return { startedAt, endedAt, progress, label };
}
