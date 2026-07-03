// 本文件把交易生命周期状态转换为参与方图、时序泳道和阶段矩阵。

import type { MatrixCell, ViewSpec } from '../../../types';
import { graphPattern, lanePattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, matrixCells } from '../runtimeView';
import type { TxLifecycleState } from './model';

/**
 * renderTxLifecycleView 基于内核状态生成交易生命周期可视化。
 */
export function renderTxLifecycleView(state: TxLifecycleState): ViewSpec {
  return { summary: `交易 ${state.txHash.slice(0, 8)},交易池${state.inMempool ? '已接收' : '未接收'},回执 ${state.receipt || '等待'}。`, patterns: [graphPattern('tx-life-graph', '交易生命周期参与方', graphNodes(state.actors), graphEdges(state.messages), 'main'), lanePattern('tx-life-lane', '交易时序', state.actors.map((actor) => actor.label), laneMessages(state.messages, (id) => labelOf(state, id)), state.tick, 'side'), matrixPattern('tx-life-matrix', '阶段状态', ['签名', '交易池', '区块', '执行', '回执'], ['结果'], txCells(state), 'bottom')] };
}

/**
 * txCells 展示交易生命周期阶段状态。
 */
function txCells(state: TxLifecycleState): MatrixCell[][] {
  const values: Record<string, boolean> = { 签名: state.signed, 交易池: state.inMempool, 区块: state.included, 执行: state.executed, 回执: state.receipt === '成功' };
  return matrixCells(['签名', '交易池', '区块', '执行', '回执'], ['结果'], (row) => ({ label: values[row] ? '完成' : state.dropped ? '失败' : '等待', status: values[row] ? 'yes' : state.dropped ? 'fault' : 'pending' }));
}

/**
 * labelOf 返回参与方展示名称。
 */
function labelOf(state: TxLifecycleState, id: string): string {
  return state.actors.find((actor) => actor.id === id)?.label ?? id;
}
