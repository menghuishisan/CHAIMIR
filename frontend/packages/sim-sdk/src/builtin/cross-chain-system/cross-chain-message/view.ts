// 本文件把跨链消息状态转换为路径图、跨链时序和状态矩阵。

import type { MatrixCell, ViewSpec } from '../../../types';
import { graphPattern, lanePattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, matrixCells } from '../crossChainView';
import type { CrossChainMessageState } from './model';

/**
 * renderCrossMessageView 基于内核状态生成跨链消息生命周期可视化。
 */
export function renderCrossMessageView(state: CrossChainMessageState): ViewSpec {
  return { summary: `消息 ${state.messageId.slice(0, 8)},验证${state.verified ? '通过' : '等待'},执行${state.executed ? '完成' : '未完成'}。`, patterns: [graphPattern('cross-message-graph', '跨链消息路径', graphNodes(state.actors), graphEdges(state.messages), 'main'), lanePattern('cross-message-lane', '跨链时序', state.actors.map((actor) => actor.label), laneMessages(state.messages, (id) => labelOf(state, id)), state.tick, 'side'), matrixPattern('cross-message-matrix', '消息状态', ['锁定', '中继', '验证', '执行'], ['结果'], messageCells(state), 'bottom')] };
}

/**
 * messageCells 展示消息阶段状态。
 */
function messageCells(state: CrossChainMessageState): MatrixCell[][] {
  const values: Record<string, boolean> = { 锁定: state.locked, 中继: state.relayed, 验证: state.verified, 执行: state.executed };
  return matrixCells(['锁定', '中继', '验证', '执行'], ['结果'], (row) => ({ label: values[row] ? '完成' : '等待', status: values[row] ? 'yes' : 'pending' }));
}

/**
 * labelOf 返回参与方展示名称。
 */
function labelOf(state: CrossChainMessageState, id: string): string {
  return state.actors.find((actor) => actor.id === id)?.label ?? id;
}
