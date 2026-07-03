// 本文件实现区块链父哈希结构的创世块、追加、分叉识别和规范链重组内核。

import type { ChainBlock, CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { stringArrayParam } from '../../initParams';
import { blockHeaderHash, dataDigest } from '../dataPrimitives';
import { blockchainPhases, type BlockchainLinkState, type BlockLink } from './model';
import { blockchainSource, traceLinesForBlockchainLink } from './trace';

/**
 * createInitialBlockchainLinkState 根据参数创建规范链。
 */
export function createInitialBlockchainLinkState(params: SimInitParams, _seed: number): BlockchainLinkState {
  const payloads = stringArrayParam(params, 'payloads', ['交易批次 A', '交易批次 B'], 1, 12, 48);
  const genesis = makeBlock(0, 'genesis', '创世块', true, false);
  const blocks = payloads.reduce<BlockLink[]>((chain, payload, index) => chain.concat(makeBlock(index + 1, chain[chain.length - 1].hash, payload, true, false)), [genesis]);
  return finalizeBlockchainLinkState({ tick: 0, phase: blockchainPhases[0].label, phaseIndex: 0, blocks, fork: [], reorganized: false, lastTransition: 'genesis', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceBlockchainLinkEvent 是父哈希结构仿真的唯一事件入口。
 */
export function reduceBlockchainLinkEvent(state: BlockchainLinkState, event: SimEvent, _context: ReducerContext): BlockchainLinkState {
  if (event.type === 'select') return finalizeBlockchainLinkState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeBlockchainLinkState(createFork(state));
  if (event.type === 'recover') return finalizeBlockchainLinkState(reorg(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeBlockchainLinkState(advanceBlockchainLink(state, event));
  return state;
}

/**
 * advanceBlockchainLink 按父哈希结构流程推进一个过程单元。
 */
export function advanceBlockchainLink(state: BlockchainLinkState, event: SimEvent): BlockchainLinkState {
  const phaseIndex = Math.min(blockchainPhases.length - 1, state.phaseIndex + 1);
  return { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: blockchainPhases[phaseIndex].id };
}

/**
 * finalizeBlockchainLinkState 刷新指标、检查点和代码追踪。
 */
export function finalizeBlockchainLinkState(state: BlockchainLinkState): BlockchainLinkState {
  const valid = state.blocks.every((block, index) => index === 0 || block.parentHash === state.blocks[index - 1].hash);
  return { ...state, phase: blockchainPhases[state.phaseIndex].label, explanation: explain(state.phaseIndex), metrics: { result: valid ? '链接有效' : '链接异常', risk: state.fork.length > 0 && !state.reorganized ? 54 : 8, height: state.blocks.length - 1 }, checkpointValues: { valid, canonical: state.fork.length === 0 }, _trace: { triggeredLines: traceLinesForBlockchainLink(state.lastTransition), variables: { height: state.blocks.length - 1, valid }, executionPath: `blockchain/${state.lastTransition}` } };
}

/**
 * blockchainLinkValid 输出父哈希结构检查点。
 */
export function blockchainLinkValid(state: BlockchainLinkState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.valid && state.checkpointValues.canonical);
  return { achieved, answer: { height: state.metrics.height, canonical: state.checkpointValues.canonical }, explanation: achieved ? '父哈希链接有效且规范链唯一。' : '仍存在链接异常或未处理分叉。' };
}

/**
 * toChainBlocks 转为链式可视化区块。
 */
export function toChainBlocks(blocks: BlockLink[]): ChainBlock[] {
  return blocks.map((block) => ({ id: block.id, height: block.height, hash: block.hash, parentHash: block.parentHash, label: block.payload, status: block.height === 0 ? 'genesis' : block.forked ? 'attacker' : block.canonical ? 'canonical' : 'pending' }));
}

/**
 * createFork 在同一父块上创建竞争分支。
 */
function createFork(state: BlockchainLinkState): BlockchainLinkState {
  const parentIndex = Math.max(0, state.blocks.length - 2);
  const parent = state.blocks[parentIndex] ?? state.blocks[0];
  const forkHead = makeBlock(parent.height + 1, parent.hash, `竞争批次 ${parent.height + 1}`, false, true);
  return { ...state, phaseIndex: 3, lastTransition: 'fork', fork: [forkHead, makeBlock(forkHead.height + 1, forkHead.hash, `竞争批次 ${forkHead.height + 1}`, false, true)], reorganized: false };
}

/**
 * reorg 将更长分叉切换为规范链并孤立旧分支。
 */
function reorg(state: BlockchainLinkState): BlockchainLinkState {
  if (state.fork.length <= 1) return { ...state, lastTransition: 'reorg' };
  const forkParentHeight = state.fork[0].height - 1;
  return { ...state, phaseIndex: 4, lastTransition: 'reorg', blocks: state.blocks.slice(0, forkParentHeight + 1).concat(state.fork.map((block) => ({ ...block, canonical: true, forked: false }))), fork: [], reorganized: true };
}

/**
 * makeBlock 创建稳定哈希区块。
 */
function makeBlock(height: number, parentHash: string, payload: string, canonical: boolean, forked: boolean): BlockLink {
  return { id: `block-${height}-${dataDigest('block-id', { payload }, 4)}`, height, payload, parentHash, hash: blockHeaderHash(height, parentHash, payload), canonical, forked };
}

/**
 * explain 生成当前阶段说明。
 */
function explain(index: number) {
  const phase = blockchainPhases[index] ?? blockchainPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}

export { blockchainSource };
