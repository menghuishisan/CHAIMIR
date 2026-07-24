// 本文件提供链上数据结构仿真共享展示辅助,只做语义数据转换,不包含具体数据结构状态机。

import type { ChartSeries } from '../../types';
export { pipelineSteps } from '../packageTools';
export { matrixCells } from '../packageTools';

/**
 * pipelineSteps 生成数据结构构建或校验流程。
 */
/**
 * matrixCells 生成结构字段校验矩阵。
 */
/**
 * metricSeries 生成一致性、风险和成本趋势。
 */
export function metricSeries(points: Array<{ x: number; consistency: number; risk: number; cost: number }>): ChartSeries[] {
  return [
    { label: '一致性', points: points.map((point) => ({ x: point.x, y: point.consistency })) },
    { label: '风险', points: points.map((point) => ({ x: point.x, y: point.risk })) },
    { label: '成本', points: points.map((point) => ({ x: point.x, y: point.cost })) },
  ];
}
