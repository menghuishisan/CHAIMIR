// 本文件实现 Merkle Tree 的排序、成对哈希、根构建、局部更新和路径重算内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { integerParam, stringArrayParam } from '../../initParams';
import { merkleRoot } from '../../merkle';
import { merkleStructureLeafHash, merkleStructureParentHash } from '../dataPrimitives';
import { merkleTreePhases, type MerkleItem, type MerkleTreeState } from './model';
import { traceLinesForMerkleTree } from './trace';

/**
 * createInitialMerkleTreeState 根据参数创建 Merkle Tree 初始叶子集合和默认更新路径。
 */
export function createInitialMerkleTreeState(params: SimInitParams, _seed: number): MerkleTreeState {
  const values = stringArrayParam(params, 'items', ['order-1', 'order-2', 'order-3', 'order-4'], 2, 16, 96);
  const targetIndex = integerParam(params, 'targetIndex', 2, 1, values.length) - 1;
  const items = values.map<MerkleItem>((value, index) => ({ id: `mtree-${index + 1}`, label: `叶子 ${index + 1}`, value, hash: merkleStructureLeafHash(index + 1, value), updated: false }));
  return finalizeMerkleTreeState({ tick: 0, phase: merkleTreePhases[0].label, phaseIndex: 0, selectedElementId: items[targetIndex].id, items, rootHash: computeMerkleRoot(items), proofPath: merkleProofPath(items, items[targetIndex].id), lastTransition: 'sort', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceMerkleTreeEvent 是 Merkle Tree 仿真的唯一事件入口。
 */
export function reduceMerkleTreeEvent(state: MerkleTreeState, event: SimEvent, _context: ReducerContext): MerkleTreeState {
  if (event.type === 'select') return finalizeMerkleTreeState({ ...state, selectedElementId: event.target, proofPath: merkleProofPath(state.items, event.target) });
  if (event.type === 'attack') return finalizeMerkleTreeState(updateLeaf(state));
  if (event.type === 'recover') return finalizeMerkleTreeState(rebuildRoot(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeMerkleTreeState(advanceMerkleTree(state, event));
  return state;
}

/**
 * advanceMerkleTree 按构树和验证流程推进一个过程单元。
 */
export function advanceMerkleTree(state: MerkleTreeState, event: SimEvent): MerkleTreeState {
  const phaseIndex = Math.min(merkleTreePhases.length - 1, state.phaseIndex + 1);
  return { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: merkleTreePhases[phaseIndex].id };
}

/**
 * finalizeMerkleTreeState 刷新指标、检查点和代码追踪。
 */
export function finalizeMerkleTreeState(state: MerkleTreeState): MerkleTreeState {
  const valid = state.rootHash === computeMerkleRoot(state.items) && !state.dirtyLeafId;
  return { ...state, phase: merkleTreePhases[state.phaseIndex].label, explanation: explain(state.phaseIndex), metrics: { result: valid ? '根摘要有效' : '等待路径重算', risk: valid ? 8 : 62, leaves: state.items.length }, checkpointValues: { rootValid: valid }, _trace: { triggeredLines: traceLinesForMerkleTree(state.lastTransition), variables: { rootHash: state.rootHash, dirtyLeafId: state.dirtyLeafId ?? '' }, executionPath: `merkle-tree/${state.lastTransition}` } };
}

/**
 * merkleTreeRootValid 输出根摘要一致性检查点。
 */
export function merkleTreeRootValid(state: MerkleTreeState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.rootValid);
  return { achieved, answer: { rootHash: state.rootHash }, explanation: achieved ? '根摘要与当前叶子集合一致。' : '根摘要尚未沿更新路径重算。' };
}

/**
 * computeMerkleRoot 计算四叶子 Merkle 根。
 */
export function computeMerkleRoot(items: MerkleItem[]): string {
  return merkleRoot(items.map((item) => item.hash), merkleStructureParentHash);
}

/**
 * updateLeaf 修改叶子并标记脏路径。
 */
function updateLeaf(state: MerkleTreeState): MerkleTreeState {
  const targetId = state.items.some((item) => item.id === state.selectedElementId) ? state.selectedElementId : state.items[1]?.id ?? state.items[0]?.id;
  return {
    ...state,
    phaseIndex: 4,
    lastTransition: 'update',
    dirtyLeafId: targetId,
    proofPath: merkleProofPath(state.items, targetId),
    items: state.items.map((item, index) => (item.id === targetId ? { ...item, value: `${item.value}-changed`, hash: merkleStructureLeafHash(index + 1, `${item.value}-changed`), updated: true } : item)),
  };
}

/**
 * rebuildRoot 沿受影响路径重算根摘要并清理脏标记。
 */
function rebuildRoot(state: MerkleTreeState): MerkleTreeState {
  const targetId = state.dirtyLeafId ?? state.selectedElementId ?? state.items[0]?.id;
  return { ...state, lastTransition: 'root', rootHash: computeMerkleRoot(state.items), proofPath: merkleProofPath(state.items, targetId), dirtyLeafId: undefined };
}

/**
 * merkleProofPath 计算叶子到根的节点 ID 路径,与树形视图的内部节点命名保持一致。
 */
export function merkleProofPath(items: MerkleItem[], targetId?: string): string[] {
  let targetIndex = Math.max(0, items.findIndex((item) => item.id === targetId));
  const path = [items[targetIndex]?.id ?? 'mtree-1'];
  let level = items.map((item) => ({ id: item.id, hash: item.hash }));
  let depth = 0;
  while (level.length > 1) {
    const padded = level.length % 2 === 0 ? level : level.concat({ ...level[level.length - 1], id: `${level[level.length - 1].id}-dup-l${depth}` });
    const next = [];
    for (let index = 0; index < padded.length; index += 2) {
      next.push({ id: Math.ceil(padded.length / 2) === 1 ? 'mtree-root' : `mtree-root-level-${depth + 1}-${index / 2}`, hash: merkleStructureParentHash(padded[index].hash, padded[index + 1].hash) });
    }
    targetIndex = Math.floor(targetIndex / 2);
    path.push(next[targetIndex].id);
    level = next;
    depth += 1;
  }
  return path;
}

/**
 * explain 生成当前阶段说明。
 */
function explain(index: number) {
  const phase = merkleTreePhases[index] ?? merkleTreePhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
