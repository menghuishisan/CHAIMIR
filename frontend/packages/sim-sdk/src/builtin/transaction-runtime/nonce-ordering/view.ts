// 本文件把 Nonce 顺序状态转换为交易队列矩阵和执行流程。

import type { MatrixCell, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../runtimeView';
import { noncePhases, type NonceState } from './model';

/**
 * renderNonceView 基于内核状态生成 Nonce 排序可视化。
 */
export function renderNonceView(state: NonceState): ViewSpec {
  return { summary: `账户 Nonce ${state.accountNonce},缺口${state.gapDetected ? '存在' : '不存在'}。`, patterns: [matrixPattern('nonce-matrix', 'Nonce 交易队列', state.txs.map((tx) => tx.id), ['Nonce', '费用', '状态'], nonceCells(state), 'main'), pipelinePattern('nonce-pipeline', 'Nonce 执行流程', pipelineSteps(noncePhases, state.phaseIndex, state.gapDetected), noncePhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * nonceCells 展示交易序号状态。
 */
function nonceCells(state: NonceState): MatrixCell[][] {
  return matrixCells(state.txs.map((tx) => tx.id), ['Nonce', '费用', '状态'], (row, column) => {
    const tx = state.txs.find((item) => item.id === row);
    if (!tx) return { label: '无', status: 'empty' };
    if (column === 'Nonce') return { label: String(tx.nonce), status: tx.nonce === state.accountNonce || tx.status === 'included' ? 'yes' : 'pending' };
    if (column === '费用') return { label: String(tx.fee), status: tx.fee >= 10 ? 'yes' : 'pending' };
    return { label: tx.status, status: tx.status === 'blocked' ? 'fault' : tx.status === 'included' ? 'yes' : 'pending' };
  });
}
