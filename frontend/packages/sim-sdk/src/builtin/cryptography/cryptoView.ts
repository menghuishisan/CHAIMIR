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
 * binaryTree 按 Merkle 成对合并规则构造任意叶子数量的二叉证明树,奇数层复制末尾节点。
 */
export function binaryTree(rootId: string, leaves: Array<{ id: string; label: string; hash: string }>, merge: (left: string, right: string) => string): TreeNode {
  let level = leaves.map<TreeNode>((leaf) => ({ id: leaf.id, label: leaf.label, hash: leaf.hash }));
  if (level.length === 0) {
    return { id: rootId, label: '根摘要', hash: '' };
  }
  let depth = 0;
  while (level.length > 1) {
    const padded = level.length % 2 === 0 ? level : level.concat({ ...level[level.length - 1], id: `${level[level.length - 1].id}-dup-l${depth}`, label: `${level[level.length - 1].label} 复制` });
    const next: TreeNode[] = [];
    for (let index = 0; index < padded.length; index += 2) {
      const pairIndex = index / 2;
      const nextLength = Math.ceil(padded.length / 2);
      next.push({
        id: nextLength === 1 ? rootId : `${rootId}-level-${depth + 1}-${pairIndex}`,
        label: nextLength === 1 ? '根摘要' : `第 ${depth + 1} 层节点 ${pairIndex + 1}`,
        hash: merge(padded[index].hash, padded[index + 1].hash),
        children: [padded[index], padded[index + 1]],
      });
    }
    level = next;
    depth += 1;
  }
  return level[0];
}
