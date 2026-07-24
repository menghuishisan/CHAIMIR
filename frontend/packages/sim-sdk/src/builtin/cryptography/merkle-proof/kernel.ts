// 本文件实现 Merkle 证明叶子哈希、兄弟路径、根重建、校验和篡改定位内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { integerParam, stringArrayParam } from '../../initParams';
import { merkleRoot } from '../../merkle';
import { foldMerkleProof, merkleLeafHash, merkleParentHash, type MerkleProofStep } from '../cryptoPrimitives';
import { merkleProofPhases, type MerkleLeaf, type MerkleProofState } from './model';
import { traceLinesForMerkleProof } from './trace';

/**
 * createInitialMerkleProofState 创建参数化 Merkle 树并为目标叶子生成完整兄弟证明路径。
 */
export function createInitialMerkleProofState(params: SimInitParams, _seed: number): MerkleProofState {
  const leafValues = stringArrayParam(params, 'leaves', ['tx:mint', 'tx:transfer', 'tx:stake', 'tx:vote'], 2, 16, 96);
  const targetIndex = integerParam(params, 'targetIndex', 2, 1, leafValues.length) - 1;
  const targetLeafId = `merkle-leaf-${targetIndex + 1}`;
  const leaves = leafValues.map<MerkleLeaf>((value, index) => ({ id: `merkle-leaf-${index + 1}`, label: `叶子 ${index + 1}`, canonicalValue: value, value, hash: merkleLeafHash(index + 1, value), inPath: index === targetIndex, tampered: false }));
  const root = rootHash(leaves);
  const proof = buildProof(leaves, targetLeafId);
  return finalizeMerkleProofState({
    tick: 0,
    phase: merkleProofPhases[0].label,
    phaseIndex: 0,
    leaves,
    targetLeafId,
    proofPath: proof.path,
    proofSiblings: proof.siblings,
    proofSteps: proof.steps,
    computedRoot: proof.root,
    expectedRoot: root,
    proofValid: true,
    lastTransition: 'leaf-hash',
    explanation: explainMerkleProofPhase(0),
    metrics: {},
    checkpointValues: {},
  });
}

/**
 * reduceMerkleProofEvent 是 Merkle 证明包唯一事件入口。
 */
export function reduceMerkleProofEvent(state: MerkleProofState, event: SimEvent, _context: ReducerContext): MerkleProofState {
  if (event.type === 'select') return finalizeMerkleProofState(selectLeaf(state, event.target));
  if (event.type === 'attack') return finalizeMerkleProofState(tamperLeaf(state));
  if (event.type === 'recover') return finalizeMerkleProofState(recoverLeaf(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeMerkleProofState(advanceMerkleProof(state, event));
  return state;
}

/**
 * advanceMerkleProof 推进 Merkle 证明验证阶段。
 */
export function advanceMerkleProof(state: MerkleProofState, event: SimEvent): MerkleProofState {
  const phaseIndex = Math.min(merkleProofPhases.length - 1, state.phaseIndex + 1);
  return rebuildProof({ ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick }, merkleProofPhases[phaseIndex].id);
}

/**
 * merkleProofValid 输出 M4 检查点可判定的 Merkle 证明结果。
 */
export function merkleProofValid(state: MerkleProofState): CheckpointResult {
  return { achieved: state.proofValid, answer: { computedRoot: state.computedRoot, expectedRoot: state.expectedRoot }, explanation: state.proofValid ? '重建根与可信根一致。' : '重建根与可信根不一致,证明失败。' };
}

/**
 * finalizeMerkleProofState 刷新教学状态、指标和代码追踪。
 */
export function finalizeMerkleProofState(state: MerkleProofState): MerkleProofState {
  return {
    ...state,
    phase: merkleProofPhases[state.phaseIndex].label,
    explanation: explainMerkleProofPhase(state.phaseIndex),
    metrics: { result: state.proofValid ? '证明通过' : '根摘要不匹配', risk: state.proofValid ? 8 : 76, pathLength: state.proofPath.length },
    checkpointValues: { proofValid: state.proofValid, target: state.targetLeafId },
    _trace: { triggeredLines: traceLinesForMerkleProof(state.lastTransition), variables: { computedRoot: state.computedRoot, expectedRoot: state.expectedRoot, proofValid: state.proofValid }, executionPath: `merkle/${state.lastTransition}` },
  };
}

/**
 * selectLeaf 选择新的目标叶子并重建路径。
 */
function selectLeaf(state: MerkleProofState, target?: string): MerkleProofState {
  if (!target) return state;
  return rebuildProof({ ...state, selectedElementId: target, targetLeafId: target, leaves: state.leaves.map((leaf) => ({ ...leaf, inPath: leaf.id === target })) }, 'path');
}

/**
 * tamperLeaf 修改目标叶子内容,使证明根不再匹配可信根。
 */
function tamperLeaf(state: MerkleProofState): MerkleProofState {
  return rebuildProof({ ...state, leaves: state.leaves.map((leaf) => (leaf.id === state.targetLeafId ? { ...leaf, value: `${leaf.value}:changed`, hash: merkleLeafHash(leafIndex(leaf.id), `${leaf.value}:changed`), tampered: true } : leaf)) }, 'tamper');
}

/**
 * recoverLeaf 恢复目标叶子的原始值,可信根保持为篡改前的链上根。
 */
function recoverLeaf(state: MerkleProofState): MerkleProofState {
  return rebuildProof(
    {
      ...state,
      leaves: state.leaves.map((leaf) => (leaf.id === state.targetLeafId ? { ...leaf, value: leaf.canonicalValue, hash: merkleLeafHash(leafIndex(leaf.id), leaf.canonicalValue), tampered: false } : leaf)),
    },
    'compare'
  );
}

/**
 * rebuildProof 重新计算证明路径和根摘要。
 */
function rebuildProof(state: MerkleProofState, transition: string): MerkleProofState {
  const proof = buildProof(state.leaves, state.targetLeafId);
  return { ...state, computedRoot: proof.root, proofValid: proof.root === state.expectedRoot, proofPath: proof.path, proofSiblings: proof.siblings, proofSteps: proof.steps, lastTransition: transition };
}

/**
 * rootHash 按标准成对合并规则计算任意叶子数量的 Merkle 根,奇数层复制末尾摘要。
 */
export function rootHash(leaves: MerkleLeaf[]): string {
  return merkleRoot(leaves.map((leaf) => leaf.hash), merkleParentHash);
}

/**
 * buildProof 只用目标叶子和兄弟摘要重建根,模拟真实 Merkle 证明验证。
 */
function buildProof(leaves: MerkleLeaf[], targetLeafId: string): { path: string[]; siblings: string[]; steps: MerkleProofStep[]; root: string } {
  let targetIndex = Math.max(0, leaves.findIndex((leaf) => leaf.id === targetLeafId));
  const path = [leaves[targetIndex]?.id ?? targetLeafId];
  const steps: MerkleProofStep[] = [];
  let level = leaves.map((leaf) => ({ id: leaf.id, hash: leaf.hash }));
  let depth = 0;
  while (level.length > 1) {
    const padded = level.length % 2 === 0 ? level : level.concat({ ...level[level.length - 1], id: `${level[level.length - 1].id}-dup-l${depth}` });
    const siblingIndex = targetIndex % 2 === 0 ? targetIndex + 1 : targetIndex - 1;
    const sibling = padded[Math.min(siblingIndex, padded.length - 1)];
    steps.push({ siblingId: sibling.id, siblingHash: sibling.hash, siblingSide: targetIndex % 2 === 0 ? 'right' : 'left' });
    const next = [];
    for (let index = 0; index < padded.length; index += 2) {
      next.push({ id: Math.ceil(padded.length / 2) === 1 ? 'merkle-root' : `merkle-root-level-${depth + 1}-${index / 2}`, hash: merkleParentHash(padded[index].hash, padded[index + 1].hash) });
    }
    targetIndex = Math.floor(targetIndex / 2);
    path.push(next[targetIndex].id);
    level = next;
    depth += 1;
  }
  return { path, siblings: steps.map((step) => `${step.siblingSide}:${step.siblingId}`), steps, root: foldMerkleProof(leaves[Math.max(0, leaves.findIndex((leaf) => leaf.id === targetLeafId))].hash, steps) };
}

/**
 * labelMerkleLeaf 返回目标叶子的展示名称。
 */
export function labelMerkleLeaf(state: MerkleProofState, id: string): string {
  return state.leaves.find((leaf) => leaf.id === id)?.label ?? id;
}

/**
 * leafIndex 从稳定叶子 id 中取回一基索引,用于摘要绑定位置。
 */
function leafIndex(id: string): number {
  const parts = id.split('-');
  return Number(parts[parts.length - 1]);
}

/**
 * explainMerkleProofPhase 生成当前阶段说明。
 */
function explainMerkleProofPhase(index: number) {
  const phase = merkleProofPhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
