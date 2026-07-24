// 本文件提供密码学仿真共享的展示数据辅助,只做语义数据转换,不包含具体密码学流程。

import type { ChartSeries, GraphNode, TreeNode } from '../../types';

export { labeledLaneMessages as laneMessages, matrixCells, messageGraphEdges as graphEdges, pipelineSteps } from '../packageTools';

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
 * pipelineSteps 根据当前阶段构造密码学流程流水线。
 */
/**
 * matrixCells 生成验证矩阵,用于展示每项约束是否通过。
 */
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
