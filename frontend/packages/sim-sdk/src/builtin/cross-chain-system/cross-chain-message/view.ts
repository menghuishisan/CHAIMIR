// 本文件把跨链消息状态转换为路径图、跨链时序和状态矩阵。

import type { MatrixCell, TeachingFrame } from '../../../types';
import { teachingFrame, graphPattern, lanePattern, matrixPattern, selectedOrFrameFocus } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, matrixCells } from '../crossChainView';
import type { CrossChainMessageState } from './model';

/**
 * renderCrossMessageView 基于内核状态生成跨链消息生命周期可视化。
 */
export function renderCrossMessageView(state: CrossChainMessageState): TeachingFrame {
  const completed = [state.locked, state.relayed, state.verified, state.executed].filter(Boolean).length;
    const summary = `消息 ${state.messageId.slice(0, 8)},生命周期 ${completed}/4,验证${state.verified ? '通过' : '等待'},执行${state.executed ? '完成' : '未完成'}。`;
  const patterns = [graphPattern('cross-message-graph', '源链锁定到目标链执行路径', graphNodes(state.actors), graphEdges(state.messages)), lanePattern('cross-message-lane', '跨链消息锁定 / 中继 / 验证 / 执行时序', state.actors.map((actor) => actor.label), laneMessages(state.messages, (id) => labelOf(state, id)), state.tick), matrixPattern('cross-message-matrix', '跨链消息生命周期状态', ['源链锁定', '中继提交', '目标链验证', '目标链执行'], ['结果'], messageCells(state))];
  return teachingFrame({
    summary,
    phase: {
      id: state.phase,
      title: state.explanation.title,
      intent: 'observe',
      what: state.explanation.effect,
      why: state.explanation.reason,
      watch: summary,
    },
    focus: {
      primary: selectedOrFrameFocus(state.selectedElementId, ['cross-message-graph']),
      secondary: ['cross-message-lane', 'cross-message-matrix'],
    },
    layout: {
      primary: 'cross-message-graph',
      evidence: ['cross-message-matrix'],
      timeline: 'cross-message-lane',
    },
    patterns,
  });
}

/**
 * messageCells 展示消息阶段状态。
 */
function messageCells(state: CrossChainMessageState): MatrixCell[][] {
  const values: Record<string, boolean> = { 源链锁定: state.locked, 中继提交: state.relayed, 目标链验证: state.verified, 目标链执行: state.executed };
  return matrixCells(['源链锁定', '中继提交', '目标链验证', '目标链执行'], ['结果'], (row) => ({ label: values[row] ? '完成' : '等待', status: values[row] ? 'yes' : 'pending' }));
}

/**
 * labelOf 返回参与方展示名称。
 */
function labelOf(state: CrossChainMessageState, id: string): string {
  return state.actors.find((actor) => actor.id === id)?.label ?? id;
}
