// 本文件提供交易运行时仿真共享展示辅助,只做交易、调用和指标语义转换。

import type { GraphNode, ProcessSpan } from '../../types';
import { pipelineSteps } from '../packageTools';

export { labeledLaneMessages as laneMessages, matrixCells, messageGraphEdges as graphEdges } from '../packageTools';

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
 * matrixCells 生成运行时检查矩阵。
 */
/**
 * pipelineSteps 生成交易运行阶段。
 */
/**
 * processRuntimeMessage 为交易运行时消息附加过程跨度。
 */
export function processRuntimeMessage(message: Omit<RuntimeMessage, 'endAt' | 'process' | 'detail'>, detail: string): RuntimeMessage {
  const endedAt = message.at + 2;
  return { ...message, endAt: endedAt, detail, process: { startedAt: message.at, endedAt, progress: message.status === 'sent' ? 0.45 : message.status === 'dropped' ? 0.75 : 1, label: detail } };
}

export { pipelineSteps };
