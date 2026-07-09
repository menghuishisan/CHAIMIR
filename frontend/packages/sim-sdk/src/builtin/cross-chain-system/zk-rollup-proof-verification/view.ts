// 本文件把 ZK Rollup 证明状态映射为证明流水线、输入矩阵和性能趋势。

import type { MatrixCell, PipelineStep, TeachingFrame, VisualElementMeta } from '../../../types';
import { chartPattern, matrixPattern, pipelinePattern, selectedOrFrameFocus, teachingFrame } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../crossChainView';
import { zkRollupPhases, type ZkRollupState } from './model';

/** renderZkRollupView 生成当前内置仿真的可视化视图模型。 */
export function renderZkRollupView(state: ZkRollupState): TeachingFrame {
  const summary = `${state.batchId} batchSize=${state.batchSize},proof=${state.proofGenerated ? '已生成' : '等待'},verifier=${state.verifierAccepted ? '接受' : state.phaseIndex === 5 ? '拒绝' : '待验证'}。`;
  const primary = state.phaseIndex === 3 || state.phaseIndex === 5 ? 'zk-rollup-matrix' : 'zk-rollup-pipeline';
  return teachingFrame({
    summary,
    phase: {
      id: zkRollupPhases[state.phaseIndex].id,
      title: state.explanation.title,
      intent: state.phaseIndex >= 3 ? 'verify' : 'observe',
      what: state.explanation.effect,
      why: state.explanation.reason,
      watch: summary,
    },
    focus: {
      primary: selectedOrFrameFocus(state.selectedElementId, [state.phaseIndex >= 3 ? 'proof' : 'zk-rollup-pipeline']),
      secondary: ['new-root', 'old-root'],
      muted: state.phaseIndex === 5 ? ['proof'] : [],
    },
    layout: { primary, evidence: ['zk-rollup-matrix'], metrics: ['zk-rollup-chart'] },
    patterns: [
      pipelinePattern('zk-rollup-pipeline', 'ZK Rollup batch -> proof -> verifier -> state root 流程', steps(state), zkRollupPhases[state.phaseIndex].id),
      matrixPattern('zk-rollup-matrix', 'proof 与 public inputs 绑定检查', state.inputs.map((input) => input.id), ['类型', '值', '一致性'], inputCells(state)),
      chartPattern('zk-rollup-chart', '批次规模、证明耗时和 L1 Gas 趋势', [
        { label: 'batchSize', points: state.history.map((point) => ({ x: point.x, y: point.batchSize })) },
        { label: 'provingTime', points: state.history.map((point) => ({ x: point.x, y: point.provingTime })) },
        { label: 'l1Gas', points: state.history.map((point) => ({ x: point.x, y: point.l1Gas })) },
      ], 'tx/s/gas'),
    ],
  });
}

/** steps 生成当前内置仿真的可视化视图模型。 */
function steps(state: ZkRollupState): PipelineStep[] {
  return pipelineSteps([...zkRollupPhases], state.phaseIndex, state.phaseIndex === 5).map((step) => ({ ...step, meta: meta(step.id, step.label, step.id === zkRollupPhases[state.phaseIndex].id ? 'focus' : step.status === 'complete' ? 'history' : 'context', state.tick) }));
}

/** inputCells 生成当前内置仿真的可视化视图模型。 */
function inputCells(state: ZkRollupState): MatrixCell[][] {
  return matrixCells(state.inputs.map((input) => input.id), ['类型', '值', '一致性'], (row, column) => {
    const input = state.inputs.find((item) => item.id === row);
    if (!input) return { label: '无', status: 'empty' };
    const cellMeta = meta(input.id, input.id, input.valid ? 'focus' : 'ghost', state.tick);
    if (column === '类型') return { label: input.kind, status: 'yes', meta: cellMeta };
    if (column === '值') return { label: input.value, status: input.valid ? 'yes' : 'pending', meta: cellMeta };
    return { label: input.valid ? '匹配' : '不匹配/等待', status: input.valid ? 'yes' : state.phaseIndex === 5 ? 'fault' : 'pending', meta: cellMeta };
  });
}

/** meta 生成当前内置仿真的可视化视图模型。 */
function meta(id: string, label: string, emphasis: VisualElementMeta['emphasis'], tick: number): VisualElementMeta {
  return { id, label, lifecycle: { state: emphasis === 'ghost' ? 'archived' : emphasis === 'history' ? 'settled' : 'active', fromTick: Math.max(0, tick - 1) }, emphasis, explanation: label };
}
