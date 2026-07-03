// 本文件实现 HotStuff 新视图、高 QC、安全投票、QC、三链提交和换主内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { bftQuorumThreshold, canonicalConsensusDigest, makeVoteCertificate } from '../consensusPrimitives';
import { processViewMessage, refreshViewMessages, type ViewMessage } from '../consensusView';
import { hotstuffPhases, type HotStuffBlock, type HotStuffState } from './model';
import { traceLinesForHotStuff } from './trace';

/**
 * createInitialHotStuffState 创建四副本 HotStuff 场景和 genesis/highQC 初始块。
 */
export function createInitialHotStuffState(_params: SimInitParams, _seed: number): HotStuffState {
  const genesis = makeHotStuffBlock('hs-genesis', undefined, 0, 'hotstuff-r1', true, true);
  return finalizeHotStuffState({
    tick: 0,
    phase: hotstuffPhases[0].label,
    phaseIndex: 0,
    view: 1,
    leaderId: 'hotstuff-r1',
    highQcBlock: genesis.id,
    proposalId: genesis.id,
    lockedBlock: genesis.id,
    replicas: ['R1', 'R2', 'R3', 'R4'].map((label, index) => ({ id: `hotstuff-r${index + 1}`, label, leader: index === 0, voted: false, lockedBlock: genesis.id, timeout: false, faulty: false })),
    blocks: [genesis],
    votes: {},
    messages: [],
    timeoutActive: false,
    lastTransition: hotstuffPhases[0].id,
    explanation: explainHotStuffPhase(0),
    metrics: {},
    checkpointValues: {},
  });
}

/**
 * reduceHotStuffEvent 是 HotStuff 仿真包唯一事件入口。
 */
export function reduceHotStuffEvent(state: HotStuffState, event: SimEvent, _context: ReducerContext): HotStuffState {
  if (event.type === 'select') return finalizeHotStuffState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeHotStuffState(injectLeaderTimeout(state));
  if (event.type === 'recover') return finalizeHotStuffState(advanceView(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeHotStuffState(advanceHotStuff(state));
  return state;
}

/**
 * advanceHotStuff 按 HotStuff 链式 BFT 流程推进一个过程单元。
 */
export function advanceHotStuff(state: HotStuffState): HotStuffState {
  const phaseIndex = Math.min(hotstuffPhases.length - 1, state.phaseIndex + (state.lastTransition === hotstuffPhases[state.phaseIndex].id ? 1 : 0));
  const next = { ...state, phaseIndex, tick: state.tick + 1 };
  if (state.phaseIndex === hotstuffPhases.length - 1) return collectNewView(advanceView({ ...state, tick: next.tick }));
  if (phaseIndex === 1) return propose(next);
  if (phaseIndex === 2) return vote(next);
  if (phaseIndex === 3) return formQc(next);
  if (phaseIndex === 4) return commitThreeChain(next);
  if (phaseIndex === 5) return state.timeoutActive ? advanceView(next) : next;
  return next;
}

/**
 * hotstuffThreeChainCommitted 检查 HotStuff 三链提交是否成立。
 */
export function hotstuffThreeChainCommitted(state: HotStuffState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.threeChain);
  return { achieved, answer: { committedBlock: state.committedBlock ?? '', highQcBlock: state.highQcBlock }, explanation: achieved ? '连续三代 QC 已形成,祖父块安全提交。' : '还没有形成可提交的三链 QC。' };
}

/**
 * hotstuffTimeoutRecovered 检查超时换主是否完成。
 */
export function hotstuffTimeoutRecovered(state: HotStuffState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.timeoutRecovered);
  return { achieved, answer: { leader: labelHotStuffReplica(state, state.leaderId), view: state.view }, explanation: achieved ? 'Pacemaker 已恢复到可继续提案的视图。' : '仍处于超时状态,需要换主。' };
}

/**
 * finalizeHotStuffState 刷新指标、消息进度、检查点和代码追踪。
 */
export function finalizeHotStuffState(state: HotStuffState): HotStuffState {
  const risk = state.timeoutActive ? 65 : state.replicas.some((replica) => replica.faulty) ? 48 : 8;
  return {
    ...state,
    phase: hotstuffPhases[state.phaseIndex].label,
    explanation: explainHotStuffPhase(state.phaseIndex),
    messages: refreshViewMessages(state.messages, state.tick, (message) => message.detail ?? `${message.label} 在当前视图内传播。`),
    metrics: { result: state.committedBlock ? '三链已提交' : '等待连续 QC', risk, votes: voteCount(state), quorum: quorum(state), view: state.view },
    checkpointValues: { threeChain: Boolean(state.committedBlock), timeoutRecovered: !state.timeoutActive },
    _trace: { triggeredLines: traceLinesForHotStuff(state.lastTransition), variables: { view: state.view, highQcBlock: state.highQcBlock, lockedBlock: state.lockedBlock, committedBlock: state.committedBlock ?? '' }, executionPath: `hotstuff/${state.lastTransition}` },
  };
}

/**
 * collectNewView 收集最高 QC 并发给当前领导者。
 */
function collectNewView(state: HotStuffState): HotStuffState {
  return { ...state, lastTransition: 'new-view', messages: state.messages.concat(state.replicas.map((replica) => message(state.tick, replica.id, state.leaderId, 'NewView'))) };
}

/**
 * propose 领导者基于 high QC 扩展新区块。
 */
function propose(state: HotStuffState): HotStuffState {
  const proposal = makeHotStuffBlock(`hs-b${state.blocks.length}`, state.highQcBlock, state.view, state.leaderId, false, false);
  return { ...state, lastTransition: 'proposal', proposalId: proposal.id, blocks: state.blocks.concat(proposal), messages: state.messages.concat(state.replicas.filter((replica) => replica.id !== state.leaderId).map((replica) => message(state.tick, state.leaderId, replica.id, 'Proposal+HighQC'))) };
}

/**
 * vote 让副本按锁规则给安全提案投票。
 */
function vote(state: HotStuffState): HotStuffState {
  const proposal = blockById(state, state.proposalId);
  const safe = Boolean(proposal && safeToVote(state, proposal));
  const votes: Record<string, string> = {};
  for (const replica of state.replicas) {
    if (!replica.faulty && safe) votes[replica.id] = state.proposalId;
  }
  return { ...state, lastTransition: 'vote', votes, replicas: state.replicas.map((replica) => ({ ...replica, voted: votes[replica.id] === state.proposalId })), messages: state.messages.concat(state.replicas.map((replica) => message(state.tick, replica.id, state.leaderId, replica.faulty ? '投票被拒绝' : '投票'))) };
}

/**
 * formQc 聚合 2f+1 投票并更新 high QC 与锁定块。
 */
function formQc(state: HotStuffState): HotStuffState {
  const signers = Object.entries(state.votes)
    .filter(([, blockId]) => blockId === state.proposalId)
    .map(([replicaId]) => replicaId);
  const certificate = makeVoteCertificate('hotstuff-qc', state.proposalId, signers, quorum(state));
  if (!certificate.achieved) return state;
  return { ...state, lastTransition: 'qc', highQcBlock: state.proposalId, lockedBlock: state.proposalId, blocks: state.blocks.map((block) => (block.id === state.proposalId ? { ...block, qc: true, qcSigners: certificate.signers, qcDigest: certificate.proofDigest } : block)), replicas: state.replicas.map((replica) => ({ ...replica, lockedBlock: state.proposalId })) };
}

/**
 * commitThreeChain 检查 proposal-parent-grandparent 是否连续带 QC 并提交祖父块。
 */
function commitThreeChain(state: HotStuffState): HotStuffState {
  const proposal = blockById(state, state.proposalId);
  const parent = proposal?.parentId ? blockById(state, proposal.parentId) : undefined;
  const grandparent = parent?.parentId ? blockById(state, parent.parentId) : undefined;
  if (!proposal?.qc || !parent?.qc || !grandparent?.qc) return { ...state, lastTransition: 'chain-commit' };
  return { ...state, lastTransition: 'chain-commit', committedBlock: grandparent.id, blocks: state.blocks.map((block) => (block.id === grandparent.id ? { ...block, committed: true } : block)) };
}

/**
 * injectLeaderTimeout 模拟领导者失效导致副本超时。
 */
function injectLeaderTimeout(state: HotStuffState): HotStuffState {
  return { ...state, tick: state.tick + 1, lastTransition: 'pacemaker', timeoutActive: true, replicas: state.replicas.map((replica) => ({ ...replica, timeout: replica.id !== state.leaderId, faulty: replica.id === state.leaderId })) };
}

/**
 * advanceView 执行 pacemaker 换主并继承最高 QC。
 */
function advanceView(state: HotStuffState): HotStuffState {
  const nextLeaderIndex = (state.replicas.findIndex((replica) => replica.id === state.leaderId) + 1) % state.replicas.length;
  const leaderId = state.replicas[nextLeaderIndex].id;
  return { ...state, lastTransition: 'new-view', phaseIndex: 0, view: state.view + 1, leaderId, timeoutActive: false, replicas: state.replicas.map((replica, index) => ({ ...replica, leader: index === nextLeaderIndex, timeout: false, faulty: false, voted: false })) };
}

/**
 * safeToVote 实现 HotStuff 锁规则:提案必须扩展锁定块,或携带更高视图 QC。
 */
function safeToVote(state: HotStuffState, proposal: HotStuffBlock): boolean {
  const locked = blockById(state, state.lockedBlock);
  const highQc = blockById(state, state.highQcBlock);
  const extendsLock = proposal.parentId === state.lockedBlock;
  const carriesFreshQc = Boolean(highQc?.qc && locked && highQc.view >= locked.view && proposal.parentId === state.highQcBlock);
  return extendsLock || carriesFreshQc;
}

/**
 * makeHotStuffBlock 创建确定性 HotStuff 区块。
 */
export function makeHotStuffBlock(id: string, parentId: string | undefined, view: number, proposerId: string, qc: boolean, committed: boolean): HotStuffBlock {
  return { id, parentId, view, proposerId, qc, committed, hash: canonicalConsensusDigest('hotstuff-block', { id, parentId: parentId ?? 'root', proposerId, view }, 12), qcSigners: qc ? [proposerId] : undefined, qcDigest: qc ? canonicalConsensusDigest('hotstuff-genesis-qc', { id, proposerId, view }, 16) : undefined };
}

/**
 * message 创建 HotStuff 协议消息。
 */
function message(at: number, from: string, to: string, label: string): ViewMessage {
  return processViewMessage(at, { id: deterministicId('hs-msg', { from, to, label, at }), from, to, label, at, status: label.includes('拒绝') ? 'dropped' : 'delivered' }, `${label} 在当前视图内传播。`);
}

/**
 * blockById 查找指定区块。
 */
function blockById(state: HotStuffState, id: string): HotStuffBlock | undefined {
  return state.blocks.find((block) => block.id === id);
}

/**
 * voteCount 统计当前提案票数。
 */
export function voteCount(state: HotStuffState): number {
  return Object.values(state.votes).filter((blockId) => blockId === state.proposalId).length;
}

/**
 * quorum 计算 HotStuff 2f+1 法定人数。
 */
export function quorum(state: HotStuffState): number {
  return bftQuorumThreshold(state.replicas.length);
}

/**
 * labelHotStuffReplica 将副本 ID 转成展示标签。
 */
export function labelHotStuffReplica(state: HotStuffState, id: string): string {
  return state.replicas.find((replica) => replica.id === id)?.label ?? id;
}

/**
 * explainHotStuffPhase 生成当前阶段说明。
 */
function explainHotStuffPhase(index: number) {
  const phase = hotstuffPhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
