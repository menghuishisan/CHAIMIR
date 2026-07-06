// 本文件把 EIP-1559 费用市场状态映射为费用流水线、交易矩阵和趋势图。

import type { MatrixCell, PipelineStep, TeachingFrame, VisualElementMeta } from '../../../types';
import { chartPattern, matrixPattern, pipelinePattern, selectedOrFrameFocus, teachingFrame } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../runtimeView';
import { feeMarketPhases, type FeeMarketState } from './model';

export function renderFeeMarketView(state: FeeMarketState): TeachingFrame {
  const included = state.transactions.filter((tx) => tx.included).length;
  const summary = `第 ${state.blockNumber} 块 base fee ${state.baseFee},已选 ${included} 笔交易,gasUsed ${state.gasUsed}/${state.targetGas},下一块 base fee ${state.nextBaseFee}。`;
  const primary = state.phaseIndex <= 3 ? 'eip1559-pipeline' : 'eip1559-chart';
  return teachingFrame({
    summary,
    phase: {
      id: feeMarketPhases[state.phaseIndex].id,
      title: state.explanation.title,
      intent: state.phaseIndex === 4 ? 'compare' : 'observe',
      what: state.explanation.effect,
      why: state.explanation.reason,
      watch: summary,
    },
    focus: {
      primary: selectedOrFrameFocus(state.selectedElementId, focusIds(state)),
      secondary: ['eip1559-matrix'],
      muted: state.transactions.filter((tx) => tx.dropped).map((tx) => tx.id),
    },
    layout: {
      primary,
      evidence: ['eip1559-matrix'],
      metrics: ['eip1559-chart'],
    },
    patterns: [
      pipelinePattern('eip1559-pipeline', 'EIP-1559 报价、打包、销毁和调价流程', steps(state), feeMarketPhases[state.phaseIndex].id),
      matrixPattern('eip1559-matrix', '交易报价与费用拆分矩阵', state.transactions.map((tx) => tx.id), ['报价', '小费', '入块', '销毁/退款'], txCells(state)),
      chartPattern('eip1559-chart', 'base fee 与区块负载趋势', [
        { label: 'base fee', points: state.history.map((point) => ({ x: point.x, y: point.baseFee })) },
        { label: 'gasUsed/target', points: state.history.map((point) => ({ x: point.x, y: Math.round((point.gasUsed / state.targetGas) * 100) })) },
      ], 'gwei/%'),
    ],
  });
}

function steps(state: FeeMarketState): PipelineStep[] {
  return pipelineSteps([...feeMarketPhases], state.phaseIndex, false).map((step) => ({ ...step, meta: meta(step.id, step.label, step.id === feeMarketPhases[state.phaseIndex].id ? 'focus' : step.status === 'complete' ? 'history' : 'context', state.tick) }));
}

function txCells(state: FeeMarketState): MatrixCell[][] {
  return matrixCells(state.transactions.map((tx) => tx.id), ['报价', '小费', '入块', '销毁/退款'], (row, column) => {
    const tx = state.transactions.find((item) => item.id === row);
    if (!tx) return { label: '无', status: 'empty' };
    const cellMeta = meta(tx.id, `${tx.sender} ${tx.id}`, tx.included ? 'focus' : tx.dropped ? 'ghost' : 'context', state.tick);
    if (column === '报价') return { label: `${tx.maxFeePerGas}`, status: tx.maxFeePerGas >= state.baseFee ? 'yes' : 'fault', meta: cellMeta };
    if (column === '小费') return { label: `${Math.max(0, Math.min(tx.maxPriorityFeePerGas, tx.maxFeePerGas - state.baseFee))}`, status: tx.maxFeePerGas >= state.baseFee ? 'yes' : 'no', meta: cellMeta };
    if (column === '入块') return { label: tx.included ? '已入块' : tx.dropped ? '低于 base fee' : '等待', status: tx.included ? 'yes' : tx.dropped ? 'fault' : 'pending', meta: cellMeta };
    return { label: tx.included ? `${tx.burned}/${tx.refunded}` : '未拆分', status: tx.included && tx.burned > 0 ? 'yes' : 'pending', meta: cellMeta };
  });
}

function focusIds(state: FeeMarketState): string[] {
  if (state.phaseIndex === 4 || state.phaseIndex === 5) return ['eip1559-chart'];
  const tx = state.transactions.find((item) => item.included) ?? state.transactions[0];
  return [tx?.id ?? 'eip1559-pipeline'];
}

function meta(id: string, label: string, emphasis: VisualElementMeta['emphasis'], tick: number): VisualElementMeta {
  return { id, label, lifecycle: { state: emphasis === 'history' ? 'settled' : emphasis === 'ghost' ? 'archived' : 'active', fromTick: Math.max(0, tick - 1) }, emphasis, explanation: label };
}
