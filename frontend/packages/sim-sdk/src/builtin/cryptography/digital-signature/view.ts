// 本文件把数字签名内核状态映射为封闭可视化模式。

import type { MatrixCell, ViewSpec } from '../../../types';
import { graphPattern, lanePattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, matrixCells } from '../cryptoView';
import type { SignatureState } from './model';

/**
 * renderDigitalSignatureView 输出参与方图、消息时序和验签条件矩阵。
 */
export function renderDigitalSignatureView(state: SignatureState): ViewSpec {
  return {
    summary: `签名 ${state.signature.slice(0, 8)},nonce ${state.nonce},验签${state.verified ? '通过' : '未通过'},重放${state.replayDetected ? '已发现' : '未发现'}。`,
    patterns: [
      graphPattern('signature-graph', '签名参与方', graphNodes(state.actors), graphEdges(state.messages), 'main'),
      lanePattern('signature-lane', '签名与验签时序', state.actors.map((actor) => actor.label), laneMessages(state.messages, (id) => labelOf(state, id)), state.tick, 'side'),
      matrixPattern('signature-matrix', '验签条件', ['恢复公钥', '消息完整', 'nonce 新鲜'], ['结果'], signatureCells(state), 'bottom'),
    ],
  };
}

/**
 * signatureCells 展示签名校验条件。
 */
function signatureCells(state: SignatureState): MatrixCell[][] {
  return matrixCells(['恢复公钥', '消息完整', 'nonce 新鲜'], ['结果'], (row) => {
    if (row === '恢复公钥') return { label: state.recoveredKey ? state.recoveredKey.slice(0, 6) : '等待', status: state.verified ? 'yes' : 'pending' };
    if (row === 'nonce 新鲜' && state.replayDetected) return { label: '已用过', status: 'fault' };
    return { label: state.verified || row !== 'nonce 新鲜' ? '通过' : '等待', status: state.verified || row !== 'nonce 新鲜' ? 'yes' : 'pending' };
  });
}

/**
 * labelOf 返回参与方标签。
 */
function labelOf(state: SignatureState, id: string): string {
  return state.actors.find((actor) => actor.id === id)?.label ?? id;
}
