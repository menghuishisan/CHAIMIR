// 本文件提供链上数据结构仿真共享展示辅助,只做语义数据转换,不包含具体数据结构状态机。

import type { ChartSeries, MatrixCell, PipelineStep, ProcessSpan } from '../../types';

/**
 * pipelineSteps 生成数据结构构建或校验流程。
 */
export function pipelineSteps(phases: Array<{ id: string; label: string; detail: string }>, activeIndex: number, failed = false): PipelineStep[] {
  return phases.map((phase, index) => ({ id: phase.id, label: phase.label, detail: phase.detail, status: index < activeIndex ? 'complete' : index === activeIndex ? (failed ? 'failed' : 'running') : 'pending', process: processSpan(index, activeIndex, phase.label) }));
}

/**
 * matrixCells 生成结构字段校验矩阵。
 */
export function matrixCells(rows: string[], columns: string[], read: (row: string, column: string) => MatrixCell): MatrixCell[][] {
  return rows.map((row) => columns.map((column) => read(row, column)));
}

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

/**
 * processSpan 给数据结构流程步骤附加过程进度,让渲染层展示连续推进而不是静态阶段名。
 */
function processSpan(index: number, activeIndex: number, label: string): ProcessSpan {
  const startedAt = index * 2;
  const endedAt = startedAt + 2;
  const progress = index < activeIndex ? 1 : index === activeIndex ? 0.58 : 0;
  return { startedAt, endedAt, progress, label };
}
