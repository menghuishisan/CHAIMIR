// 本文件把 EVM 调用栈状态转换为调用图、调用时序和栈帧矩阵。

import type { MatrixCell, ViewSpec } from '../../../types';
import { graphPattern, lanePattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, matrixCells } from '../runtimeView';
import type { CallStackState } from './model';

/**
 * renderCallStackView 基于内核状态生成 EVM 调用栈可视化。
 */
export function renderCallStackView(state: CallStackState): ViewSpec {
  return { summary: `活跃栈深 ${state.frames.filter((frame) => !frame.returned).length}/${state.maxDepth},回滚失败帧 ${state.frames.filter((frame) => frame.reverted).length}。`, patterns: [graphPattern('call-stack-graph', '合约调用图', graphNodes(state.actors), graphEdges(state.messages), 'main'), lanePattern('call-stack-lane', '调用栈时序', state.actors.map((actor) => actor.label), laneMessages(state.messages, (id) => labelOf(state, id)), state.tick, 'side'), matrixPattern('call-stack-matrix', '栈帧状态', state.frames.map((frame) => frame.id), ['合约', '深度', '返回', '回滚失败'], stackCells(state), 'bottom')] };
}

/**
 * stackCells 展示栈帧状态。
 */
function stackCells(state: CallStackState): MatrixCell[][] {
  return matrixCells(state.frames.map((frame) => frame.id), ['合约', '深度', '返回', '回滚失败'], (row, column) => {
    const frame = state.frames.find((item) => item.id === row);
    if (!frame) return { label: '无', status: 'empty' };
    if (column === '合约') return { label: frame.contract, status: 'yes' };
    if (column === '深度') return { label: String(frame.depth), status: frame.depth > state.maxDepth ? 'fault' : 'yes' };
    if (column === '返回') return { label: frame.returned ? '已返回' : '等待', status: frame.returned ? 'yes' : 'pending' };
    return { label: frame.reverted ? '失败' : '否', status: frame.reverted ? 'fault' : 'empty' };
  });
}

/**
 * labelOf 返回参与方展示名称。
 */
function labelOf(state: CallStackState, id: string): string {
  return state.actors.find((actor) => actor.id === id)?.label ?? id;
}
