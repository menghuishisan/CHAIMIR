// 本文件把 Optimistic Rollup 欺诈证明状态映射为流程、争议树和 L1 链状态。

import type { PipelineStep, TeachingFrame, VisualElementMeta } from '../../../types';
import { chainPattern, pipelinePattern, selectedOrFrameFocus, teachingFrame, treePattern } from '../../packageTools';
import { pipelineSteps } from '../crossChainView';
import { disputeTree, optimisticRollupChain } from './kernel';
import { optimisticRollupPhases, type OptimisticRollupState } from './model';

export function renderOptimisticRollupView(state: OptimisticRollupState): TeachingFrame {
  const summary = `${state.batchId} claimedRoot=${state.claimedRoot},expectedRoot=${state.expectedRoot},挑战${state.challenged ? '已开启' : '未开启'},欺诈${state.fraudProven ? '成立' : '未裁决'}。`;
  const primary = state.phaseIndex >= 3 && state.phaseIndex <= 4 ? 'op-rollup-dispute' : 'op-rollup-pipeline';
  return teachingFrame({
    summary,
    phase: {
      id: optimisticRollupPhases[state.phaseIndex].id,
      title: state.explanation.title,
      intent: state.phaseIndex >= 2 ? 'attack' : 'observe',
      what: state.explanation.effect,
      why: state.explanation.reason,
      watch: summary,
    },
    focus: {
      primary: selectedOrFrameFocus(state.selectedElementId, state.phaseIndex >= 3 ? ['l2tx-2'] : [state.batchId]),
      secondary: ['op-rollup-chain'],
      muted: state.fraudProven ? [state.batchId] : [],
    },
    layout: { primary, evidence: ['op-rollup-chain'], timeline: 'op-rollup-pipeline' },
    patterns: [
      pipelinePattern('op-rollup-pipeline', 'L2 batch -> L1 challenge -> fraud proof 流程', steps(state), optimisticRollupPhases[state.phaseIndex].id),
      treePattern('op-rollup-dispute', '交互式二分争议执行 trace', disputeTree(state), ['trace-root', 'right-half', 'l2tx-2']),
      chainPattern('op-rollup-chain', 'L1 上 batch 状态', optimisticRollupChain(state)),
    ],
  });
}

function steps(state: OptimisticRollupState): PipelineStep[] {
  return pipelineSteps([...optimisticRollupPhases], state.phaseIndex, state.fraudProven && state.phaseIndex === 5).map((step) => ({ ...step, meta: meta(step.id, step.label, step.id === optimisticRollupPhases[state.phaseIndex].id ? 'focus' : step.status === 'complete' ? 'history' : 'context', state.tick) }));
}

function meta(id: string, label: string, emphasis: VisualElementMeta['emphasis'], tick: number): VisualElementMeta {
  return { id, label, lifecycle: { state: emphasis === 'history' ? 'settled' : 'active', fromTick: Math.max(0, tick - 1) }, emphasis, explanation: label };
}
