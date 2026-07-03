// 本文件把区块链父哈希结构状态转换为链、矩阵和流程三种语义可视化。

import type { MatrixCell, ViewSpec } from '../../../types';
import { chainPattern, matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../dataView';
import { toChainBlocks } from './kernel';
import { blockchainPhases, type BlockchainLinkState } from './model';

/**
 * renderBlockchainLinkView 基于内核状态生成父哈希结构可视化。
 */
export function renderBlockchainLinkView(state: BlockchainLinkState): ViewSpec {
  return { summary: `高度 ${state.blocks.length - 1},分叉 ${state.fork.length} 块,重组${state.reorganized ? '已完成' : '未发生'}。`, patterns: [chainPattern('blockchain-chain', '区块链与分叉', toChainBlocks(state.blocks), state.fork.length > 0 ? [toChainBlocks(state.fork)] : [], 'main'), matrixPattern('blockchain-matrix', '链接校验', state.blocks.map((block) => `高度 ${block.height}`), ['父哈希', '规范链', '分叉'], blockCells(state), 'side'), pipelinePattern('blockchain-pipeline', '结构校验流程', pipelineSteps(blockchainPhases, state.phaseIndex, state.fork.length > 0 && !state.reorganized), blockchainPhases[state.phaseIndex].id, 'bottom')] };
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
