// 本文件提供合约安全仿真共享展示辅助,只做参与方、调用和检查矩阵语义转换。

import type { GraphEdge, GraphNode, LaneMessage, MatrixCell, PipelineStep, ProcessSpan } from '../../types';

export interface SecurityActor {
  id: string;
  label: string;
  role: string;
  status: 'idle' | 'active' | 'success' | 'warning' | 'danger';
  value?: string;
}

export interface SecurityCall {
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
 * graphNodes 将合约安全参与方映射为图节点。
 */
export function graphNodes(actors: SecurityActor[]): GraphNode[] {
  return actors;
}

/**
 * graphEdges 将调用或资金流映射为图边。
 */
export function graphEdges(calls: SecurityCall[]): GraphEdge[] {
  return calls.map((call) => ({ id: call.id, from: call.from, to: call.to, label: call.label, status: call.status === 'dropped' ? 'failed' : call.status === 'delivered' ? 'success' : 'active', process: call.process, detail: call.detail }));
}

/**
 * laneMessages 把内部 ID 转为泳道展示名称。
 */
export function laneMessages(calls: SecurityCall[], labelOf: (id: string) => string): LaneMessage[] {
  return calls.map((call) => ({ ...call, from: labelOf(call.from), to: labelOf(call.to) }));
}

/**
 * matrixCells 生成漏洞检查矩阵。
 */
export function matrixCells(rows: string[], columns: string[], read: (row: string, column: string) => MatrixCell): MatrixCell[][] {
  return rows.map((row) => columns.map((column) => read(row, column)));
}

/**
 * pipelineSteps 生成漏洞利用或修复阶段。
 */
export function pipelineSteps(phases: Array<{ id: string; label: string; detail: string }>, activeIndex: number, failed = false): PipelineStep[] {
  return phases.map((phase, index) => ({ id: phase.id, label: phase.label, detail: phase.detail, status: index < activeIndex ? 'complete' : index === activeIndex ? (failed ? 'failed' : 'running') : 'pending', process: processSpan(index, activeIndex, phase.label) }));
}

/**
 * processSecurityCall 为合约调用附加过程跨度,让图和泳道展示真实调用推进。
 */
export function processSecurityCall(call: Omit<SecurityCall, 'endAt' | 'process' | 'detail'>, detail: string): SecurityCall {
  const endedAt = call.at + 2;
  return { ...call, endAt: endedAt, detail, process: { startedAt: call.at, endedAt, progress: call.status === 'sent' ? 0.45 : call.status === 'dropped' ? 0.78 : 1, label: detail } };
}

/**
 * processSpan 给安全流程步骤附加过程进度。
 */
function processSpan(index: number, activeIndex: number, label: string): ProcessSpan {
  const startedAt = index * 2;
  const endedAt = startedAt + 2;
  const progress = index < activeIndex ? 1 : index === activeIndex ? 0.6 : 0;
  return { startedAt, endedAt, progress, label };
}
