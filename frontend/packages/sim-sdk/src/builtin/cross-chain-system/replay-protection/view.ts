// 本文件把跨链重放防护状态转换为字段矩阵和防护流程。

import type { MatrixCell, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../crossChainView';
import { replayPhases, type ReplayState } from './model';

/**
 * renderReplayView 基于内核状态生成重放防护可视化。
 */
export function renderReplayView(state: ReplayState): ViewSpec {
  const nonceSeen = state.executedNonces.includes(state.nonce);
  return { summary: `Domain ${state.domain},Nonce ${state.nonce},历史命中${nonceSeen ? '是' : '否'},状态 ${state.accepted ? '接受' : state.replayAttempt ? '拒绝重放' : '等待'}。`, patterns: [matrixPattern('replay-matrix', 'Domain + Nonce + MessageHash 防重放键', ['Domain', 'Nonce', '已执行集合', '消息哈希'], ['结果'], replayCells(state), 'main'), pipelinePattern('replay-pipeline', '构造唯一键 -> 查重 -> 记录执行流程', pipelineSteps(replayPhases, state.phaseIndex, state.replayAttempt), replayPhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * replayCells 展示防重放字段。
 */
function replayCells(state: ReplayState): MatrixCell[][] {
  return matrixCells(['Domain', 'Nonce', '已执行集合', '消息哈希'], ['结果'], (row) => {
    if (row === 'Domain') return { label: state.domain, status: 'yes' };
    if (row === 'Nonce') return { label: String(state.nonce), status: state.executedNonces.includes(state.nonce) ? 'fault' : 'yes' };
    if (row === '已执行集合') return { label: state.executedNonces.join(',') || '空', status: state.executedNonces.includes(state.nonce) ? 'fault' : 'pending' };
    return { label: state.messageHash.slice(0, 8), status: 'yes' };
  });
}
