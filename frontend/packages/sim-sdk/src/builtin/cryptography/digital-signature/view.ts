// 本文件把数字签名内核状态映射为封闭可视化模式。

import type { MatrixCell, TeachingFrame } from '../../../types';
import { teachingFrame, graphPattern, lanePattern, matrixPattern, selectedOrFrameFocus } from '../../packageTools';
import { graphEdges, graphNodes, laneMessages, matrixCells } from '../cryptoView';
import type { SignatureState } from './model';

/**
 * renderDigitalSignatureView 输出参与方图、消息时序和验签条件矩阵。
 */
export function renderDigitalSignatureView(state: SignatureState): TeachingFrame {
  const replayState = state.replayDetected ? 'Nonce 重放' : 'Nonce 新鲜';
    const summary = `签名 ${state.signature.slice(0, 8)},Nonce ${state.nonce},${replayState},恢复公钥 ${state.recoveredKey ? state.recoveredKey.slice(0, 6) : '等待'},验签${state.verified ? '通过' : '未通过'}。`;
  const patterns = [
      graphPattern('signature-graph', '签名者 -> 验证者信任链路', graphNodes(state.actors), graphEdges(state.messages)),
      lanePattern('signature-lane', '签名生成 / 公钥恢复 / Nonce 检查时序', state.actors.map((actor) => actor.label), laneMessages(state.messages, (id) => labelOf(state, id)), state.tick),
      matrixPattern('signature-matrix', 'ECDSA 验签条件矩阵', ['恢复公钥', '消息完整', 'Nonce 未重放'], ['结果'], signatureCells(state)),
    ];
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
      primary: selectedOrFrameFocus(state.selectedElementId, ['signature-graph']),
      secondary: ['signature-lane', 'signature-matrix'],
    },
    layout: {
      primary: 'signature-graph',
      evidence: ['signature-matrix'],
      timeline: 'signature-lane',
    },
    patterns,
  });
}

/**
 * signatureCells 展示签名校验条件。
 */
function signatureCells(state: SignatureState): MatrixCell[][] {
  return matrixCells(['恢复公钥', '消息完整', 'Nonce 未重放'], ['结果'], (row) => {
    if (row === '恢复公钥') return { label: state.recoveredKey ? state.recoveredKey.slice(0, 6) : '等待', status: state.verified ? 'yes' : 'pending' };
    if (row === 'Nonce 未重放' && state.replayDetected) return { label: '已用过', status: 'fault' };
    return { label: state.verified || row !== 'Nonce 未重放' ? '通过' : '等待', status: state.verified || row !== 'Nonce 未重放' ? 'yes' : 'pending' };
  });
}

/**
 * labelOf 返回参与方标签。
 */
function labelOf(state: SignatureState, id: string): string {
  return state.actors.find((actor) => actor.id === id)?.label ?? id;
}
