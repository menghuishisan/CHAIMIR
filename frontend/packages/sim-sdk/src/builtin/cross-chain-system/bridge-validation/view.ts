// 本文件把跨链桥验证状态转换为检查矩阵和验证流程。

import type { MatrixCell, ViewSpec } from '../../../types';
import { matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../crossChainView';
import { bridgePhases, type BridgeState } from './model';

/**
 * renderBridgeView 基于内核状态生成跨链桥验证可视化。
 */
export function renderBridgeView(state: BridgeState): ViewSpec {
  return { summary: `证明 ${state.proofHash.slice(0, 8)},轻客户端${state.lightClientSynced ? '已同步' : '未同步'},铸造${state.minted ? '完成' : '等待'}。`, patterns: [matrixPattern('bridge-matrix', '桥验证状态', ['轻客户端', '包含证明', '铸造', '赎回'], ['结果'], bridgeCells(state), 'main'), pipelinePattern('bridge-pipeline', '桥验证流程', pipelineSteps(bridgePhases, state.phaseIndex, state.invalidProof), bridgePhases[state.phaseIndex].id, 'bottom')] };
}

/**
 * bridgeCells 展示桥验证检查项。
 */
function bridgeCells(state: BridgeState): MatrixCell[][] {
  const values: Record<string, boolean> = { 轻客户端: state.lightClientSynced, 包含证明: !state.invalidProof && state.lightClientSynced, 铸造: state.minted, 赎回: state.redeemed };
  return matrixCells(['轻客户端', '包含证明', '铸造', '赎回'], ['结果'], (row) => ({ label: values[row] ? '通过' : state.invalidProof && row === '包含证明' ? '失败' : '等待', status: values[row] ? 'yes' : state.invalidProof && row === '包含证明' ? 'fault' : 'pending' }));
}
