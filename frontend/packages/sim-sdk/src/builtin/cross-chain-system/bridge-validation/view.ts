// 本文件把跨链桥验证状态转换为检查矩阵和验证流程。

import type { MatrixCell, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../crossChainView';
import { bridgePhases, type BridgeState } from './model';

/**
 * renderBridgeView 基于内核状态生成跨链桥验证可视化。
 */
export function renderBridgeView(state: BridgeState): ViewSpec {
  const proofMatched = state.proofHash === state.canonicalProofHash && !state.invalidProof;
  return { summary: `提交证明 ${state.proofHash.slice(0, 8)},规范证明 ${state.canonicalProofHash.slice(0, 8)},证明${proofMatched ? '匹配' : '不匹配'},轻客户端${state.lightClientSynced ? '已同步' : '未同步'},铸造${state.minted ? '完成' : '等待'}。`, patterns: [matrixPattern('bridge-matrix', '跨链桥信任根与包含证明矩阵', ['轻客户端信任根', '锁仓包含证明', '目标链铸造', '反向赎回'], ['结果'], bridgeCells(state), 'main'), pipelinePattern('bridge-pipeline', '锁仓 -> 轻客户端同步 -> 证明验证 -> 铸造/赎回', pipelineSteps(bridgePhases, state.phaseIndex, state.invalidProof), bridgePhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * bridgeCells 展示桥验证检查项。
 */
function bridgeCells(state: BridgeState): MatrixCell[][] {
  const proofMatched = state.proofHash === state.canonicalProofHash && !state.invalidProof;
  const values: Record<string, boolean> = { 轻客户端信任根: state.lightClientSynced, 锁仓包含证明: proofMatched && state.lightClientSynced, 目标链铸造: state.minted, 反向赎回: state.redeemed };
  return matrixCells(['轻客户端信任根', '锁仓包含证明', '目标链铸造', '反向赎回'], ['结果'], (row) => ({ label: values[row] ? '通过' : row === '锁仓包含证明' && !proofMatched ? '哈希不匹配' : '等待', status: values[row] ? 'yes' : row === '锁仓包含证明' && !proofMatched ? 'fault' : 'pending' }));
}
