// 本文件提供密码学仿真共享的展示数据辅助,只做语义数据转换,不包含具体密码学流程。

import type { ChartSeries, GraphEdge, GraphNode, LaneMessage, MatrixCell, PipelineStep, TreeNode } from '../../types';

export interface CryptoActor {
  id: string;
  label: string;
  role: string;
  status: 'idle' | 'active' | 'success' | 'warning' | 'danger';
  value?: string;
}

export interface CryptoMessage {
  id: string;
  from: string;
  to: string;
  at: number;
  label: string;
  status: 'sent' | 'delivered' | 'dropped';
}

/**
 * graphNodes 将密码学参与方映射为图节点。
 */
export function graphNodes(actors: CryptoActor[]): GraphNode[] {
  return actors;
}

/**
 * graphEdges 将证明、签名或密钥消息映射为图边。
 */
export function graphEdges(messages: CryptoMessage[]): GraphEdge[] {
  return messages.map((message) => ({
    id: message.id,
    from: message.from,
    to: message.to,
    label: message.label,
    status: message.status === 'dropped' ? 'failed' : message.status === 'delivered' ? 'success' : 'active',
  }));
}

/**
 * laneMessages 把内部 ID 转成时序泳道展示名称。
 */
export function laneMessages(messages: CryptoMessage[], labelOf: (id: string) => string): LaneMessage[] {
  return messages.map((message) => ({ ...message, from: labelOf(message.from), to: labelOf(message.to) }));
}

/**
 * pipelineSteps 根据当前阶段构造密码学流程流水线。
 */
export function pipelineSteps(phases: Array<{ id: string; label: string; detail: string }>, activeIndex: number, failed = false): PipelineStep[] {
  return phases.map((phase, index) => ({
    id: phase.id,
    label: phase.label,
    detail: phase.detail,
    status: index < activeIndex ? 'complete' : index === activeIndex ? (failed ? 'failed' : 'running') : 'pending',
  }));
}

/**
 * matrixCells 生成验证矩阵,用于展示每项约束是否通过。
 */
export function matrixCells(rows: string[], columns: string[], read: (row: string, column: string) => MatrixCell): MatrixCell[][] {
  return rows.map((row) => columns.map((column) => read(row, column)));
}

/**
 * metricSeries 生成密码学流程常用的正确性、风险和成本曲线。
 */
export function metricSeries(points: Array<{ x: number; correctness: number; risk: number; cost: number }>): ChartSeries[] {
  return [
    { label: '正确性', points: points.map((point) => ({ x: point.x, y: point.correctness })) },
    { label: '风险', points: points.map((point) => ({ x: point.x, y: point.risk })) },
    { label: '成本', points: points.map((point) => ({ x: point.x, y: point.cost })) },
  ];
}

/**
 * binaryTree 用叶子和合并函数构造二叉证明树。
 */
export function binaryTree(rootId: string, leaves: Array<{ id: string; label: string; hash: string }>, merge: (left: string, right: string) => string): TreeNode {
  const leafNodes = leaves.map<TreeNode>((leaf) => ({ id: leaf.id, label: leaf.label, hash: leaf.hash }));
  const leftHash = merge(leafNodes[0].hash, leafNodes[1].hash);
  const rightHash = merge(leafNodes[2].hash, leafNodes[3].hash);
  return {
    id: rootId,
    label: '根摘要',
    hash: merge(leftHash, rightHash),
    children: [
      { id: `${rootId}-left`, label: '左分支', hash: leftHash, children: leafNodes.slice(0, 2) },
      { id: `${rootId}-right`, label: '右分支', hash: rightHash, children: leafNodes.slice(2) },
    ],
  };
}
