// 本文件把 Ethereum PoS 最终性状态映射为链、投票矩阵和最终性趋势。

import type { MatrixCell, TeachingFrame, VisualElementMeta } from '../../../types';
import { chainPattern, chartPattern, matrixPattern, selectedOrFrameFocus, teachingFrame } from '../../packageTools';
import { trendSeries, voteCells } from '../consensusView';
import { ethPosChainBlocks } from './kernel';
import { ethPosFinalityPhases, type EthPosFinalityState } from './model';

export function renderEthPosFinalityView(state: EthPosFinalityState): TeachingFrame {
  const summary = `Slot ${state.slot},head=${state.head},justified=${state.justified},finalized=${state.finalized},在线验证者 ${state.validators.filter((v) => v.online).length}/${state.validators.length}。`;
  const primary = state.phaseIndex <= 2 ? 'eth-pos-chain' : state.phaseIndex <= 4 ? 'eth-pos-votes' : 'eth-pos-chart';
  return teachingFrame({
    summary,
    phase: {
      id: ethPosFinalityPhases[state.phaseIndex].id,
      title: state.explanation.title,
      intent: state.phaseIndex >= 3 ? 'verify' : 'observe',
      what: state.explanation.effect,
      why: state.explanation.reason,
      watch: summary,
    },
    focus: {
      primary: selectedOrFrameFocus(state.selectedElementId, [state.head]),
      secondary: [state.justified, state.finalized],
      muted: state.blocks.filter((block) => block.status === 'orphaned').map((block) => block.id),
    },
    layout: { primary, evidence: ['eth-pos-votes'], metrics: ['eth-pos-chart'] },
    patterns: [
      chainPattern('eth-pos-chain', 'LMD-GHOST 链头与 checkpoint', ethPosChainBlocks(state)),
      matrixPattern('eth-pos-votes', '验证者最新消息和 FFG 权重', state.validators.map((validator) => validator.label), ['权重', '在线', 'latest vote', 'finality'], voteMatrix(state)),
      chartPattern('eth-pos-chart', '参与率、风险和最终性趋势', trendSeries(state.participationHistory), '%'),
    ],
  });
}

function voteMatrix(state: EthPosFinalityState): MatrixCell[][] {
  return voteCells(state.validators.map((validator) => validator.label), ['权重', '在线', 'latest vote', 'finality'], (row, column) => {
    const validator = state.validators.find((item) => item.label === row);
    if (!validator) return { label: '无', status: 'empty' };
    const cellMeta = meta(validator.id, validator.label, validator.latestVote === state.head ? 'focus' : 'context', state.tick);
    if (column === '权重') return { label: String(validator.weight), status: 'yes', meta: cellMeta };
    if (column === '在线') return { label: validator.online ? '在线' : '延迟', status: validator.online ? 'yes' : 'fault', meta: cellMeta };
    if (column === 'latest vote') return { label: validator.latestVote ?? '未投', status: validator.latestVote ? 'yes' : 'pending', meta: cellMeta };
    return { label: state.finalized !== 'genesis' ? 'finalized' : state.justified !== 'b0' ? 'justified' : '等待', status: state.finalized !== 'genesis' ? 'yes' : 'pending', meta: cellMeta };
  });
}

function meta(id: string, label: string, emphasis: VisualElementMeta['emphasis'], tick: number): VisualElementMeta {
  return { id, label, lifecycle: { state: emphasis === 'focus' ? 'active' : 'settled', fromTick: Math.max(0, tick - 1) }, emphasis, explanation: label };
}
