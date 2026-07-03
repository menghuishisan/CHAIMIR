// 本文件把哈希链内核状态映射为封闭可视化模式,不包含状态迁移逻辑。

import type { ChainBlock, MatrixCell, ViewSpec } from '../../../types';
import { chainPattern, matrixPattern, pipelinePattern } from '../../packageTools';
import { matrixCells, pipelineSteps } from '../cryptoView';
import { hashChainPhases, type HashChainState } from './model';

/**
 * renderHashChainView 输出哈希链、校验矩阵和验证流程。
 */
export function renderHashChainView(state: HashChainState): ViewSpec {
  const failed = state.records.some((record) => !record.valid);
  return {
    summary: `当前阶段 ${state.phase},无效记录 ${state.records.filter((record) => !record.valid).length} 条。`,
    patterns: [
      chainPattern('hash-chain', '哈希链', chainBlocks(state), [], 'main'),
      matrixPattern('hash-matrix', '校验矩阵', state.records.map((record) => `记录 ${record.index}`), ['载荷', '父哈希', '摘要'], hashCells(state), 'side'),
      pipelinePattern('hash-pipeline', '哈希验证流程', pipelineSteps([...hashChainPhases], state.phaseIndex, failed), hashChainPhases[state.phaseIndex].id, 'bottom'),
    ],
  };
}

/**
 * chainBlocks 把哈希记录映射为链式区块。
 */
function chainBlocks(state: HashChainState): ChainBlock[] {
  return state.records.map((record) => ({ id: record.id, height: record.index, hash: record.hash, parentHash: record.parentHash, label: `记录 ${record.index}`, status: record.index === 1 ? 'genesis' : record.tampered ? 'attacker' : record.valid ? 'canonical' : 'orphaned' }));
}

/**
 * hashCells 展示载荷、父哈希和摘要是否通过校验。
 */
function hashCells(state: HashChainState): MatrixCell[][] {
  return matrixCells(
    state.records.map((record) => `记录 ${record.index}`),
    ['载荷', '父哈希', '摘要'],
    (row, column) => {
      const record = state.records.find((item) => row.endsWith(String(item.index)));
      if (!record) return { label: '无', status: 'empty' };
      if (record.tampered && column === '载荷') return { label: '被改动', status: 'fault' };
      if (!record.valid && column !== '载荷') return { label: '不匹配', status: 'fault' };
      return { label: column === '载荷' ? '已规范' : '通过', status: 'yes' };
    }
  );
}
