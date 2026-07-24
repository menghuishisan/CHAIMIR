// 本文件提供合约安全仿真共享展示辅助,只做参与方、调用和检查矩阵语义转换。

import type { GraphNode, ProcessSpan } from '../../types';
import { pipelineSteps as buildPipelineSteps } from '../packageTools';

export { labeledLaneMessages as laneMessages, matrixCells, messageGraphEdges as graphEdges } from '../packageTools';

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
 * matrixCells 生成漏洞检查矩阵。
 */
/**
 * pipelineSteps 生成漏洞利用或修复阶段。
 */
/**
 * processSecurityCall 为合约调用附加过程跨度,让图和泳道展示真实调用推进。
 */
export function processSecurityCall(call: Omit<SecurityCall, 'endAt' | 'process' | 'detail'>, detail: string): SecurityCall {
  const endedAt = call.at + 2;
  return { ...call, endAt: endedAt, detail, process: { startedAt: call.at, endedAt, progress: call.status === 'sent' ? 0.45 : call.status === 'dropped' ? 0.78 : 1, label: detail } };
}

export { buildPipelineSteps as pipelineSteps };
