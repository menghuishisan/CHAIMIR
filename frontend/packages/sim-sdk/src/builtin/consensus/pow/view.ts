// 本文件把 PoW 内核状态映射为封闭可视化模式,不包含协议状态迁移。

import type { ChainBlock, MatrixCell, ViewSpec } from '../../../types';
import { chainPattern, graphPattern, matrixPattern } from '../../packageTools';
import { graphEdges, graphNodes, voteCells, type ViewNode } from '../consensusView';
import { canonicalHeight, chainWork } from './kernel';
import type { PowBlock, PowState } from './model';

/**
 * renderPowView 输出规范链、矿工网络和 nonce 搜索过程。
 */
export function renderPowView(state: PowState): ViewSpec {
  const privateWork = chainWork(state.privateFork);
  const publicWork = chainWork(state.blocks);
  const winningAttempts = state.hashAttempts.filter((attempt) => attempt.valid).length;
  return {
    summary: `高度 ${canonicalHeight(state)},目标 ${state.targetPrefix},候选 nonce ${state.candidateNonce},命中 ${winningAttempts} 次,规范链工作量 ${publicWork},私有分叉工作量 ${privateWork}。`,
    patterns: [
      chainPattern('pow-chain', `PoW 最长工作量链,公开 ${publicWork} / 私有 ${privateWork}`, chainBlocks(state.blocks), state.privateFork.length > 0 ? [chainBlocks(state.privateFork)] : [], 'main'),
      graphPattern('pow-graph', '矿工算力与区块广播网络', minerNodes(state), graphEdges(state.messages), 'side'),
      matrixPattern('pow-attempts', `Nonce 搜索窗口,难度 ${state.difficulty} 个前导零`, state.hashAttempts.map((attempt) => String(attempt.nonce)), ['哈希前缀', '前导零得分', '是否达标'], powAttemptCells(state), 'bottom'),
    ],
  };
}

/**
 * minerNodes 把矿工算力和攻击状态映射为图节点。
 */
function minerNodes(state: PowState): ViewNode[] {
  return graphNodes(state.miners.map((miner) => ({ id: miner.id, label: miner.label, role: 'miner', status: miner.attacker ? 'danger' : miner.accepted ? 'success' : 'warning', value: `算力 ${miner.hashPower}%` })));
}

/**
 * chainBlocks 将内部区块转换为链式可视化结构。
 */
function chainBlocks(blocks: PowBlock[]): ChainBlock[] {
  return blocks.map((item) => ({ id: item.id, height: item.height, hash: item.hash, parentHash: item.parentHash, label: item.height === 0 ? '创世块' : `高度 ${item.height}`, status: item.height === 0 ? 'genesis' : item.attacker ? 'attacker' : item.canonical ? 'canonical' : 'pending' }));
}

/**
 * powAttemptCells 展示最近 nonce 尝试和目标匹配结果。
 */
function powAttemptCells(state: PowState): MatrixCell[][] {
  return voteCells(
    state.hashAttempts.map((attempt) => String(attempt.nonce)),
    ['哈希前缀', '前导零得分', '是否达标'],
    (row, column) => {
      const attempt = state.hashAttempts.find((item) => String(item.nonce) === row);
      if (!attempt) return { label: '无', status: 'empty' };
      if (column === '哈希前缀') return { label: attempt.hash.slice(0, 8), status: attempt.valid ? 'yes' : 'pending' };
      if (column === '前导零得分') return { label: `${attempt.score}/${state.difficulty}`, status: attempt.score >= state.difficulty ? 'yes' : 'pending' };
      return { label: state.targetPrefix, status: attempt.valid ? 'yes' : 'fault' };
    }
  );
}
