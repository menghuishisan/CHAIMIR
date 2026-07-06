// 本文件把区块链父哈希结构状态转换为链、矩阵和流程三种语义可视化。

import type { MatrixCell, TeachingFrame } from '../../../types';
import { teachingFrame, chainPattern, matrixPattern, pipelinePattern, selectedOrFrameFocus } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../dataView';
import { toChainBlocks } from './kernel';
import { blockchainPhases, type BlockchainLinkState } from './model';

/**
 * renderBlockchainLinkView 基于内核状态生成父哈希结构可视化。
 */
export function renderBlockchainLinkView(state: BlockchainLinkState): TeachingFrame {
  const canonicalTip = state.blocks[state.blocks.length - 1];
  const forkTip = state.fork[state.fork.length - 1];
    const summary = `规范高度 ${canonicalTip?.height ?? 0},分叉高度 ${forkTip?.height ?? 0},分叉 ${state.fork.length} 块,重组${state.reorganized ? '已完成' : '未发生'}。`;
  const patterns = [chainPattern('blockchain-chain', '父哈希链接、分叉与重组路径', toChainBlocks(state.blocks), state.fork.length > 0 ? [toChainBlocks(state.fork)] : []), matrixPattern('blockchain-matrix', '父哈希连续性与规范链选择', state.blocks.map((block) => `高度 ${block.height}`), ['父哈希', '规范链', '分叉'], blockCells(state)), pipelinePattern('blockchain-pipeline', '父哈希校验 -> 分叉检测 -> 重组流程', pipelineSteps(blockchainPhases, state.phaseIndex, state.fork.length > 0 && !state.reorganized), blockchainPhases[state.phaseIndex].id)];
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
      primary: selectedOrFrameFocus(state.selectedElementId, ['blockchain-chain']),
      secondary: ['blockchain-matrix', 'blockchain-pipeline'],
    },
    layout: {
      primary: 'blockchain-chain',
      evidence: ['blockchain-matrix', 'blockchain-pipeline'],
    },
    patterns,
  });
}

/**
 * blockCells 展示父哈希、规范链和分叉状态。
 */
function blockCells(state: BlockchainLinkState): MatrixCell[][] {
  return matrixCells(state.blocks.map((block) => `高度 ${block.height}`), ['父哈希', '规范链', '分叉'], (row, column) => {
    const height = Number(row.replace('高度 ', ''));
    const block = state.blocks.find((item) => item.height === height);
    if (!block) return { label: '无', status: 'empty' };
    if (column === '父哈希') return { label: block.parentHash.slice(0, 6), status: 'yes' };
    if (column === '规范链') return { label: block.canonical ? '是' : '否', status: block.canonical ? 'yes' : 'pending' };
    return { label: block.forked ? '是' : '否', status: block.forked ? 'fault' : 'empty' };
  });
}
