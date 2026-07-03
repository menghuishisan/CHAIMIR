// 本文件实现 Merkle Tree 的排序、成对哈希、根构建、局部更新和路径重算内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { merkleStructureLeafHash, merkleStructureParentHash } from '../dataPrimitives';
import { merkleTreePhases, type MerkleItem, type MerkleTreeState } from './model';
import { traceLinesForMerkleTree } from './trace';

/**
 * createInitialMerkleTreeState 创建四叶子 Merkle Tree 初始状态。
 */
export function createInitialMerkleTreeState(_params: SimInitParams, _seed: number): MerkleTreeState {
  const items = ['order-1', 'order-2', 'order-3', 'order-4'].map<MerkleItem>((value, index) => ({ id: `mtree-${index + 1}`, label: `叶子 ${index + 1}`, value, hash: merkleStructureLeafHash(index + 1, value), updated: false }));
  return finalizeMerkleTreeState({ tick: 0, phase: merkleTreePhases[0].label, phaseIndex: 0, items, rootHash: computeMerkleRoot(items), proofPath: ['mtree-2', 'mtree-left', 'mtree-root'], lastTransition: 'sort', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceMerkleTreeEvent 是 Merkle Tree 仿真的唯一事件入口。
 */
export function reduceMerkleTreeEvent(state: MerkleTreeState, event: SimEvent, _context: ReducerContext): MerkleTreeState {
  if (event.type === 'select') return finalizeMerkleTreeState({ ...state, selectedElementId: event.target });
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
  const left = merkleStructureParentHash(items[0].hash, items[1].hash);
  const right = merkleStructureParentHash(items[2].hash, items[3].hash);
  return merkleStructureParentHash(left, right);
}

/**
 * updateLeaf 修改叶子并标记脏路径。
 */
function updateLeaf(state: MerkleTreeState): MerkleTreeState {
  return { ...state, phaseIndex: 4, lastTransition: 'update', dirtyLeafId: 'mtree-2', items: state.items.map((item, index) => (item.id === 'mtree-2' ? { ...item, value: `${item.value}-changed`, hash: merkleStructureLeafHash(index + 1, `${item.value}-changed`), updated: true } : item)) };
}

/**
 * rebuildRoot 沿受影响路径重算根摘要并清理脏标记。
 */
function rebuildRoot(state: MerkleTreeState): MerkleTreeState {
  return { ...state, lastTransition: 'root', rootHash: computeMerkleRoot(state.items), dirtyLeafId: undefined };
}

/**
 * explain 生成当前阶段说明。
 */
function explain(index: number) {
  const phase = merkleTreePhases[index] ?? merkleTreePhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
