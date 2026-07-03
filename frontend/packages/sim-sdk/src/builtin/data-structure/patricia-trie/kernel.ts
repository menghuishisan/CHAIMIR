// 本文件实现 Patricia Trie 的路径编码、压缩路径、局部更新、根哈希和缺失证明内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { trieLeafHash, trieRootHash } from '../dataPrimitives';
import { patriciaTriePhases, type PatriciaTrieState, type TrieEntry } from './model';
import { traceLinesForPatriciaTrie } from './trace';

/**
 * createInitialPatriciaTrieState 创建三条账户路径和一个缺失证明 key。
 */
export function createInitialPatriciaTrieState(_params: SimInitParams, _seed: number): PatriciaTrieState {
  const entries = ['alice', 'alina', 'bob'].map<TrieEntry>((key, index) => ({ key, path: encodeTrieKey(key), value: String((index + 1) * 10), hash: leafHash(key, String((index + 1) * 10)), updated: false, missing: false }));
  return finalizePatriciaTrieState({ tick: 0, phase: patriciaTriePhases[0].label, phaseIndex: 0, entries, rootHash: computeTrieRoot(entries), proofKey: 'alex', proofValid: true, lastTransition: 'encode', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reducePatriciaTrieEvent 是 Patricia Trie 仿真的唯一事件入口。
 */
export function reducePatriciaTrieEvent(state: PatriciaTrieState, event: SimEvent, _context: ReducerContext): PatriciaTrieState {
  if (event.type === 'select') return finalizePatriciaTrieState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizePatriciaTrieState(updateWrongLeaf(state));
  if (event.type === 'recover') return finalizePatriciaTrieState(rehashTrie(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizePatriciaTrieState(advancePatriciaTrie(state, event));
  return state;
}

/**
 * advancePatriciaTrie 按 Trie 更新流程推进一个过程单元。
 */
export function advancePatriciaTrie(state: PatriciaTrieState, event: SimEvent): PatriciaTrieState {
  const phaseIndex = Math.min(patriciaTriePhases.length - 1, state.phaseIndex + 1);
  const next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: patriciaTriePhases[phaseIndex].id };
  return phaseIndex >= 3 ? rehashTrie(next) : next;
}

/**
 * finalizePatriciaTrieState 刷新指标、检查点和代码追踪。
 */
export function finalizePatriciaTrieState(state: PatriciaTrieState): PatriciaTrieState {
  return { ...state, phase: patriciaTriePhases[state.phaseIndex].label, explanation: explain(state.phaseIndex), metrics: { result: state.proofValid ? '根哈希有效' : '等待重算', risk: state.proofValid ? 8 : 68, entries: state.entries.length }, checkpointValues: { rootValid: state.rootHash === computeTrieRoot(state.entries), proofValid: state.proofValid }, _trace: { triggeredLines: traceLinesForPatriciaTrie(state.lastTransition), variables: { rootHash: state.rootHash, proofKey: state.proofKey }, executionPath: `patricia-trie/${state.lastTransition}` } };
}

/**
 * patriciaTrieValid 输出 Trie 根哈希与缺失证明检查点。
 */
export function patriciaTrieValid(state: PatriciaTrieState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.rootValid && state.checkpointValues.proofValid);
  return { achieved, answer: { rootHash: state.rootHash, proofKey: state.proofKey }, explanation: achieved ? '根哈希与路径证明一致。' : '根哈希或缺失证明仍不一致。' };
}

/**
 * encodeTrieKey 生成教学用 nibble 路径。
 */
export function encodeTrieKey(key: string): string {
  return key.split('').map((char) => char.charCodeAt(0).toString(16)).join('');
}

/**
 * leafHash 计算叶子哈希。
 */
export function leafHash(key: string, value: string): string {
  return trieLeafHash(key, encodeTrieKey(key), value);
}

/**
 * computeTrieRoot 计算 Trie 根哈希。
 */
export function computeTrieRoot(entries: TrieEntry[]): string {
  return trieRootHash(entries.map((entry) => ({ path: entry.path, hash: entry.hash })));
}

/**
 * updateWrongLeaf 修改叶子但暂不更新根哈希。
 */
function updateWrongLeaf(state: PatriciaTrieState): PatriciaTrieState {
  return { ...state, phaseIndex: 2, lastTransition: 'insert', entries: state.entries.map((entry) => (entry.key === 'alice' ? { ...entry, value: '99', hash: leafHash(entry.key, '99'), updated: true } : entry)), proofValid: false };
}

/**
 * rehashTrie 自底向上重算根哈希。
 */
function rehashTrie(state: PatriciaTrieState): PatriciaTrieState {
  return { ...state, lastTransition: state.lastTransition === 'insert' ? 'rehash' : state.lastTransition, rootHash: computeTrieRoot(state.entries), proofValid: true };
}

/**
 * explain 生成当前阶段说明。
 */
function explain(index: number) {
  const phase = patriciaTriePhases[index] ?? patriciaTriePhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
