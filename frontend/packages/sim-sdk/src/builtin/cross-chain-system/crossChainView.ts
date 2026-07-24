// 本文件提供跨链系统仿真共享展示辅助,只做链、委员会和消息语义转换。

import type { GraphNode, ProcessSpan } from '../../types';
import { pipelineSteps as buildPipelineSteps } from '../packageTools';

export { labeledLaneMessages as laneMessages, matrixCells, messageGraphEdges as graphEdges } from '../packageTools';

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
 * matrixCells 生成跨链检查矩阵。
 */
/**
 * pipelineSteps 生成跨链流程阶段。
 */
/**
 * processCrossMessage 为跨链消息附加过程跨度。
 */
export function processCrossMessage(message: Omit<CrossMessage, 'endAt' | 'process' | 'detail'>, detail: string): CrossMessage {
  const endedAt = message.at + 3;
  return { ...message, endAt: endedAt, detail, process: { startedAt: message.at, endedAt, progress: message.status === 'sent' ? 0.45 : message.status === 'dropped' ? 0.75 : 1, label: detail } };
}

export { buildPipelineSteps as pipelineSteps };
