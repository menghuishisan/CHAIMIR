// 本文件把 mempool 替换交易状态映射为泳道、矩阵和传播图。

import type { GraphNode, MatrixCell, TeachingFrame, VisualElementMeta } from '../../../types';
import { graphPattern, lanePattern, matrixPattern, selectedOrFrameFocus, teachingFrame } from '../../packageTools';
import { graphEdges, laneMessages, matrixCells } from '../runtimeView';
import { labelPoolActor } from './kernel';
import { mempoolReplacementPhases, type MempoolReplacementState, type PoolStatus } from './model';

export function renderMempoolReplacementView(state: MempoolReplacementState): TeachingFrame {
  const pending = state.transactions.filter((tx) => tx.status === 'pending').length;
  const queued = state.transactions.filter((tx) => tx.status === 'queued').length;
  const summary = `交易池 pending ${pending} 笔,queued ${queued} 笔,替换阈值 ${state.replacementRequiredBump}%,Alice expectedNonce=${state.expectedNonce.Alice}。`;
  const primary = state.phaseIndex === 3 ? 'mempool-graph' : state.phaseIndex <= 2 ? 'mempool-matrix' : 'mempool-lane';
  return teachingFrame({
    summary,
    phase: {
      id: mempoolReplacementPhases[state.phaseIndex].id,
      title: state.explanation.title,
      intent: state.phaseIndex === 2 ? 'verify' : 'observe',
      what: state.explanation.effect,
      why: state.explanation.reason,
      watch: summary,
    },
    focus: {
      primary: selectedOrFrameFocus(state.selectedElementId, focusIds(state)),
      secondary: ['mempool-matrix'],
      muted: state.transactions.filter((tx) => tx.status === 'replaced' || tx.status === 'rejected').map((tx) => tx.id),
    },
    layout: { primary, evidence: ['mempool-matrix'], timeline: 'mempool-lane' },
    patterns: [
      lanePattern('mempool-lane', '交易提交、传播和打包时序', ['用户', '本地节点', '对等节点', '构建器', '区块'], laneMessages(state.messages, labelPoolActor), state.tick),
      matrixPattern('mempool-matrix', '账户 nonce 队列与替换结果', state.transactions.map((tx) => tx.id), ['账户/nonce', '费用', '状态', '原因'], poolCells(state)),
      graphPattern('mempool-graph', '节点间 mempool 视图传播', graphNodes(state), graphEdges(state.messages)),
    ],
  });
}

function poolCells(state: MempoolReplacementState): MatrixCell[][] {
  return matrixCells(state.transactions.map((tx) => tx.id), ['账户/nonce', '费用', '状态', '原因'], (row, column) => {
    const tx = state.transactions.find((item) => item.id === row);
    if (!tx) return { label: '无', status: 'empty' };
    const cellMeta = meta(tx.id, tx.id, emphasisFor(tx.status), state.tick);
    if (column === '账户/nonce') return { label: `${tx.account} #${tx.nonce}`, status: 'yes', meta: cellMeta };
    if (column === '费用') return { label: String(tx.fee), status: tx.status === 'rejected' ? 'fault' : 'yes', meta: cellMeta };
    if (column === '状态') return { label: statusLabel(tx.status), status: statusCell(tx.status), meta: cellMeta };
    return { label: tx.reason, status: statusCell(tx.status), meta: cellMeta };
  });
}

function graphNodes(state: MempoolReplacementState): GraphNode[] {
  const seenByNode = new Map(state.nodeViews.map((nodeView) => [nodeView.node, nodeView.seen.length]));
  return ['user', 'local', 'peer', 'builder', 'block'].map((id) => ({
    id,
    label: labelPoolActor(id),
    role: id === 'block' ? 'chain' : 'mempool',
    status: id === 'local' ? 'active' : id === 'block' && state.transactions.some((tx) => tx.status === 'included') ? 'success' : 'idle',
    value: id === 'local' ? `${seenByNode.get('本地节点') ?? state.transactions.length} tx` : id === 'peer' ? `${seenByNode.get('对等节点') ?? 0} tx` : id === 'builder' ? `${seenByNode.get('构建器') ?? 0} tx` : undefined,
    meta: meta(id, labelPoolActor(id), id === 'local' ? 'focus' : 'context', state.tick),
  }));
}

function focusIds(state: MempoolReplacementState): string[] {
  const active = state.transactions.find((tx) => tx.status === 'pending') ?? state.transactions[0];
  return [active?.id ?? 'mempool-matrix'];
}

function statusLabel(status: PoolStatus): string {
  return ({ pending: 'pending', queued: 'queued', replaced: '已替换', rejected: '已拒绝', included: '已入块' })[status];
}

function statusCell(status: PoolStatus): MatrixCell['status'] {
  if (status === 'pending' || status === 'included') return 'yes';
  if (status === 'rejected') return 'fault';
  if (status === 'replaced') return 'no';
  return 'pending';
}

function emphasisFor(status: PoolStatus): VisualElementMeta['emphasis'] {
  if (status === 'pending' || status === 'included') return 'focus';
  if (status === 'replaced' || status === 'rejected') return 'ghost';
  return 'context';
}

function meta(id: string, label: string, emphasis: VisualElementMeta['emphasis'], tick: number): VisualElementMeta {
  return { id, label, lifecycle: { state: emphasis === 'ghost' ? 'archived' : emphasis === 'focus' ? 'active' : 'settled', fromTick: Math.max(0, tick - 1) }, emphasis, explanation: label };
}
