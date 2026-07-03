// 本文件提供交易运行时仿真共享展示辅助,只做交易、调用和指标语义转换。

import type { GraphEdge, GraphNode, LaneMessage, MatrixCell, PipelineStep, ProcessSpan } from '../../types';

export interface RuntimeActor {
  id: string;
  label: string;
  role: string;
  status: 'idle' | 'active' | 'success' | 'warning' | 'danger';
  value?: string;
}

export interface RuntimeMessage {
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
 * graphNodes 将运行时参与方转换为图节点。
 */
export function graphNodes(actors: RuntimeActor[]): GraphNode[] {
  return actors;
}

/**
 * graphEdges 将交易、调用或验证消息转换为图边。
 */
export function graphEdges(messages: RuntimeMessage[]): GraphEdge[] {
  return messages.map((message) => ({ id: message.id, from: message.from, to: message.to, label: message.label, status: message.status === 'dropped' ? 'failed' : message.status === 'delivered' ? 'success' : 'active', process: message.process, detail: message.detail }));
}

/**
 * laneMessages 把内部 ID 转成时序泳道名称。
 */
export function laneMessages(messages: RuntimeMessage[], labelOf: (id: string) => string): LaneMessage[] {
  return messages.map((message) => ({ ...message, from: labelOf(message.from), to: labelOf(message.to) }));
}

/**
 * matrixCells 生成运行时检查矩阵。
 */
export function matrixCells(rows: string[], columns: string[], read: (row: string, column: string) => MatrixCell): MatrixCell[][] {
  return rows.map((row) => columns.map((column) => read(row, column)));
}

/**
 * pipelineSteps 生成交易运行阶段。
 */
export function pipelineSteps(phases: Array<{ id: string; label: string; detail: string }>, activeIndex: number, failed = false): PipelineStep[] {
  return phases.map((phase, index) => ({ id: phase.id, label: phase.label, detail: phase.detail, status: index < activeIndex ? 'complete' : index === activeIndex ? (failed ? 'failed' : 'running') : 'pending', process: processSpan(index, activeIndex, phase.label) }));
}

/**
 * processRuntimeMessage 为交易运行时消息附加过程跨度。
 */
export function processRuntimeMessage(message: Omit<RuntimeMessage, 'endAt' | 'process' | 'detail'>, detail: string): RuntimeMessage {
  const endedAt = message.at + 2;
  return { ...message, endAt: endedAt, detail, process: { startedAt: message.at, endedAt, progress: message.status === 'sent' ? 0.45 : message.status === 'dropped' ? 0.75 : 1, label: detail } };
}

/**
 * processSpan 给交易运行阶段附加过程进度。
 */
function processSpan(index: number, activeIndex: number, label: string): ProcessSpan {
  const startedAt = index * 2;
  const endedAt = startedAt + 2;
  const progress = index < activeIndex ? 1 : index === activeIndex ? 0.6 : 0;
  return { startedAt, endedAt, progress, label };
}
