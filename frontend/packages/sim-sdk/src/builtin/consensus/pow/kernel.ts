// 本文件实现 PoW 最长链共识内核,渲染层只能消费这里产出的确定性状态。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { indexFromSeed, integerParam, weightedShares } from '../../initParams';
import { canonicalConsensusDigest } from '../consensusPrimitives';
import { processViewMessage, refreshViewMessages, type ViewMessage } from '../consensusView';
import { powPhases, type PowAttempt, type PowBlock, type PowState } from './model';
import { traceLinesForPow } from './trace';

/**
 * createInitialPowState 根据参数构造矿工集合、难度、内存池和确定性候选区块。
 */
export function createInitialPowState(params: SimInitParams, seed: number): PowState {
  const difficulty = integerParam(params, 'difficulty', 4, 2, 5);
  const mempoolSize = integerParam(params, 'mempoolSize', 42, 1, 5000);
  const hashWindowSize = integerParam(params, 'hashWindowSize', 2048, 128, 8192);
  const privateForkTargetDepth = integerParam(params, 'privateForkTargetDepth', 2, 1, 6);
  const targetSpacing = integerParam(params, 'targetSpacing', 6, 2, 60);
  const minerCount = integerParam(params, 'minerCount', 3, 3, 8);
  const genesis = createPowBlock({ id: 'pow-genesis', height: 0, minerId: 'network', parentHash: 'genesis', difficulty: 0, nonce: 0, mempoolSize: 0, hash: '0000000000000000', attacker: false, canonical: true });
  const miners = createPowMiners(params, minerCount, genesis.hash);
  const initialMiner = miners[indexFromSeed(seed, miners.length)] ?? miners[0];
  return finalizePowState({
    tick: 0,
    phase: powPhases[0].label,
    phaseIndex: 0,
    difficulty,
    targetPrefix: targetPrefix(difficulty),
    mempoolSize,
    hashWindowSize,
    candidateNonce: 0,
    candidateHash: hashPowCandidate(genesis.hash, 1, initialMiner.id, 0, mempoolSize),
    candidateReady: false,
    candidateParentHash: genesis.hash,
    candidateMinerId: initialMiner.id,
    hashAttempts: [],
    targetSpacing,
    miners,
    blocks: [genesis],
    privateFork: [],
    privateForkTargetDepth,
    privateMiningCursor: 0,
    privateMiningTargetDepth: 0,
    messages: [],
    samples: [{ x: 0, quorum: 30, risk: 12, finality: 15 }],
    selfishMining: false,
    lastTransition: powPhases[0].id,
    explanation: explainPowPhase(0),
    metrics: {},
    checkpointValues: {},
  });
}

/**
 * reducePowEvent 是 PoW 仿真包唯一事件入口,保持状态迁移可回放。
 */
export function reducePowEvent(state: PowState, event: SimEvent, _context: ReducerContext): PowState {
  if (event.type === 'select') return finalizePowState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizePowState(startSelfishMining(state));
  if (event.type === 'recover') return finalizePowState(publishPrivateFork(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizePowState(advancePow(state));
  return state;
}

/**
 * advancePow 按 PoW 协议顺序推进一个内核过程单元。
 */
export function advancePow(state: PowState): PowState {
  if (state.selfishMining && state.privateFork.length < state.privateMiningTargetDepth) {
    return continueSelfishMining({ ...state, tick: state.tick + 1 });
  }
  if (state.phaseIndex === 2 && state.lastTransition === 'hash-search' && !state.candidateReady) {
    return searchNonce({ ...state, phaseIndex: 2, tick: state.tick + 1 });
  }
  const phaseIndex = Math.min(powPhases.length - 1, state.phaseIndex + (state.lastTransition === powPhases[state.phaseIndex].id ? 1 : 0));
  const base = { ...state, phaseIndex, tick: state.tick + 1 };
  if (phaseIndex === 1) return assembleCandidate(base);
  if (phaseIndex === 2) return searchNonce(base);
  if (phaseIndex === 3) return broadcastBlock(base);
  if (phaseIndex === 4) return validateBlock(base);
  if (phaseIndex === 5) return chooseLongestChain(base);
  if (phaseIndex === 6) return adjustDifficulty(base);
  return base;
}

/**
 * powWorkValid 检查非创世区块是否满足当前难度下的工作量声明。
 */
export function powWorkValid(state: PowState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.workValid);
  return { achieved, answer: { difficulty: state.difficulty, tipWork: state.blocks[state.blocks.length - 1]?.work }, explanation: achieved ? '所有非创世区块都满足工作量目标。' : '存在工作量不足的区块。' };
}

/**
 * powForkChoiceValid 检查规范链是否至少拥有最高累计工作量。
 */
export function powForkChoiceValid(state: PowState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.forkChoice);
  return { achieved, answer: { canonicalWork: chainWork(state.blocks), forkWork: forkChainWork(state) }, explanation: achieved ? '节点选择了累计工作量最高的链。' : '私有分叉工作量更高,需要处理重组。' };
}

/**
 * finalizePowState 刷新指标、检查点、历史样本和代码追踪。
 */
export function finalizePowState(state: PowState): PowState {
  const risk = state.selfishMining ? 72 : state.privateFork.length > 0 ? 48 : 14;
  const finality = Math.min(96, state.blocks.length * 18 - (state.selfishMining ? 20 : 0));
  const samples = state.samples.concat({ x: state.tick + state.phaseIndex, quorum: Math.min(100, chainWork(state.blocks) * 8), risk, finality }).slice(-24);
  const nonGenesisBlocks = state.blocks.concat(state.privateFork).filter((item) => item.height > 0);
  return {
    ...state,
    phase: powPhases[state.phaseIndex].label,
    explanation: explainPowPhase(state.phaseIndex),
    messages: refreshViewMessages(state.messages, state.tick, (message) => message.detail ?? `${message.label} 从矿工传播到对端节点。`),
    samples,
    metrics: { result: state.selfishMining && state.privateFork.length < state.privateMiningTargetDepth ? '私有分叉继续挖矿' : state.selfishMining ? '存在私有分叉' : state.phaseIndex === 2 && !state.candidateReady ? '继续搜索 nonce' : '按累计工作量收敛', risk, finality, work: chainWork(state.blocks), difficulty: state.difficulty },
    checkpointValues: { workValid: nonGenesisBlocks.every((item) => blockValid(item)), forkChoice: chainWork(state.blocks) >= forkChainWork(state) },
    _trace: { triggeredLines: traceLinesForPow(state.lastTransition), variables: { difficulty: state.difficulty, target: state.targetPrefix, nonce: state.candidateNonce, candidateHash: state.candidateHash, attempts: state.hashAttempts.length, candidateReady: state.candidateReady, privateDepth: state.privateFork.length, hashWindowSize: state.hashWindowSize }, executionPath: `pow/${state.lastTransition}` },
  };
}

/**
 * assembleCandidate 绑定当前规范链尖和内存池交易数量。
 */
function assembleCandidate(state: PowState): PowState {
  const parent = state.blocks[state.blocks.length - 1];
  const miner = selectMiner(state);
  const candidateNonce = state.candidateNonce + 1;
  const candidateHash = hashPowCandidate(parent.hash, parent.height + 1, miner.id, candidateNonce, state.mempoolSize);
  const attempt = { nonce: candidateNonce, hash: candidateHash, score: leadingZeroNibbles(candidateHash), valid: blockHashValid(candidateHash, state.difficulty) };
  return { ...state, lastTransition: 'assemble', candidateParentHash: parent.hash, candidateMinerId: miner.id, candidateNonce, candidateHash, candidateReady: attempt.valid, hashAttempts: [attempt] };
}

/**
 * searchNonce 按配置窗口枚举 nonce,未命中时停留在哈希搜索阶段等待下一次推进。
 */
function searchNonce(state: PowState): PowState {
  const parent = state.blocks[state.blocks.length - 1];
  const search = mineCandidateWindow(parent, state.candidateMinerId, state.mempoolSize, state.difficulty, state.candidateNonce + 1, state.hashWindowSize);
  return { ...state, phaseIndex: search.found ? state.phaseIndex : 2, lastTransition: 'hash-search', candidateNonce: search.nonce, candidateHash: search.hash, candidateReady: search.found, hashAttempts: search.attempts };
}

/**
 * broadcastBlock 将有效新区块追加到规范链并广播给所有矿工。
 */
function broadcastBlock(state: PowState): PowState {
  const parent = state.blocks[state.blocks.length - 1];
  const expectedHash = hashPowCandidate(parent.hash, parent.height + 1, state.candidateMinerId, state.candidateNonce, state.mempoolSize);
  if (!state.candidateReady || state.candidateHash !== expectedHash || !blockHashValid(state.candidateHash, state.difficulty)) {
    return searchNonce({ ...state, phaseIndex: 2 });
  }
  const mined = createPowBlock({ id: `pow-block-${state.blocks.length}`, height: parent.height + 1, minerId: state.candidateMinerId, parentHash: parent.hash, difficulty: state.difficulty, nonce: state.candidateNonce, mempoolSize: state.mempoolSize, hash: state.candidateHash, attacker: false, canonical: true });
  return { ...state, lastTransition: 'broadcast', candidateReady: false, blocks: state.blocks.concat(mined), messages: state.messages.concat(broadcast(state, state.candidateMinerId, '新区块')) };
}

/**
 * validateBlock 让每个矿工独立验证区块并更新本地链尖。
 */
function validateBlock(state: PowState): PowState {
  const tip = state.blocks[state.blocks.length - 1];
  const parentKnown = state.blocks.some((block) => block.hash === tip.parentHash) || tip.height === 0;
  const validWork = blockValid(tip);
  return { ...state, lastTransition: 'validate', miners: state.miners.map((miner) => ({ ...miner, validTip: parentKnown && validWork ? tip.hash : miner.validTip, accepted: parentKnown && validWork })) };
}

/**
 * chooseLongestChain 比较规范链与私有分叉的累计工作量并选择高工作量链。
 */
function chooseLongestChain(state: PowState): PowState {
  if (forkChainWork(state) > chainWork(state.blocks)) {
    return { ...state, lastTransition: 'longest-chain', blocks: adoptPrivateFork(state), privateFork: [], privateMiningTargetDepth: 0, selfishMining: false };
  }
  return { ...state, lastTransition: 'longest-chain', privateMiningTargetDepth: state.privateFork.length >= state.privateMiningTargetDepth ? 0 : state.privateMiningTargetDepth };
}

/**
 * adjustDifficulty 根据区块增长速度调整教学难度目标。
 */
function adjustDifficulty(state: PowState): PowState {
  const window = state.blocks.filter((block) => block.height > 0).slice(-4);
  const observedSpacing = window.length > 1 ? Math.max(1, state.tick / window.length) : state.targetSpacing;
  const nextDifficulty = observedSpacing < state.targetSpacing * 0.75 ? Math.min(5, state.difficulty + 1) : observedSpacing > state.targetSpacing * 1.5 ? Math.max(2, state.difficulty - 1) : state.difficulty;
  return { ...state, lastTransition: 'adjust', difficulty: nextDifficulty, targetPrefix: targetPrefix(nextDifficulty) };
}

/**
 * startSelfishMining 进入私有挖矿模式,分叉区块仍必须通过真实 PoW 搜索产生。
 */
function startSelfishMining(state: PowState): PowState {
  const attacker = attackerMiner(state);
  return continueSelfishMining({
    ...state,
    tick: state.tick + 1,
    lastTransition: 'selfish-mining',
    selfishMining: true,
    privateMiningCursor: Math.max(state.privateMiningCursor, state.candidateNonce + 31),
    privateMiningTargetDepth: Math.max(state.privateForkTargetDepth, state.privateMiningTargetDepth),
    miners: state.miners.map((miner) => (miner.id === attacker.id ? { ...miner, attacker: true } : miner)),
  });
}

/**
 * publishPrivateFork 发布私有分叉,让最长链规则显式处理重组。
 */
function publishPrivateFork(state: PowState): PowState {
  if (state.privateFork.length === 0) {
    return continueSelfishMining({ ...state, tick: state.tick + 1 });
  }
  return chooseLongestChain({ ...state, tick: state.tick + 1, messages: state.messages.concat(broadcast(state, attackerMiner(state).id, '发布私有分叉')) });
}

/**
 * createPowBlock 创建确定性 PoW 区块。
 */
export function createPowBlock(input: { id: string; height: number; minerId: string; parentHash: string; difficulty: number; nonce: number; mempoolSize: number; hash: string; attacker: boolean; canonical: boolean }): PowBlock {
  return { id: input.id, height: input.height, minerId: input.minerId, parentHash: input.parentHash, hash: input.hash, nonce: input.nonce, mempoolSize: input.mempoolSize, difficulty: input.difficulty, work: workForDifficulty(input.difficulty), canonical: input.canonical, attacker: input.attacker };
}

/**
 * broadcast 生成矿工广播消息。
 */
function broadcast(state: PowState, from: string, label: string): ViewMessage[] {
  return state.miners
    .filter((miner) => miner.id !== from)
    .map((miner) =>
      processViewMessage(state.tick, { id: deterministicId('pow-msg', { from, to: miner.id, label, tick: state.tick }), from, to: miner.id, at: state.tick, label, status: 'delivered' }, `${label} 从矿工传播到对端节点。`)
    );
}

/**
 * chainWork 计算链的累计工作量。
 */
export function chainWork(blocks: PowBlock[]): number {
  return blocks.reduce((sum, item) => sum + item.work, 0);
}

/**
 * forkChainWork 计算私有分叉加共同祖先后的累计工作量。
 */
function forkChainWork(state: PowState): number {
  if (state.privateFork.length === 0) return 0;
  return chainWork(commonPrefixForFork(state).concat(state.privateFork));
}

/**
 * adoptPrivateFork 保留共同祖先并把有效私有分支切换为规范链。
 */
function adoptPrivateFork(state: PowState): PowBlock[] {
  return commonPrefixForFork(state)
    .concat(state.privateFork)
    .map((item) => ({ ...item, canonical: true }));
}

/**
 * commonPrefixForFork 找到私有分叉的共同祖先前缀。
 */
function commonPrefixForFork(state: PowState): PowBlock[] {
  const baseHash = state.privateFork[0]?.parentHash;
  const baseIndex = state.blocks.findIndex((block) => block.hash === baseHash);
  return baseIndex >= 0 ? state.blocks.slice(0, baseIndex + 1) : [];
}

/**
 * canonicalHeight 返回规范链当前高度。
 */
export function canonicalHeight(state: PowState): number {
  return state.blocks[state.blocks.length - 1]?.height ?? 0;
}

/**
 * explainPowPhase 生成当前阶段说明。
 */
function explainPowPhase(index: number) {
  const phase = powPhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}

/**
 * selectMiner 按算力权重确定本轮公开链出块矿工,避免使用随机源导致回放漂移。
 */
function selectMiner(state: PowState) {
  const active = state.miners.filter((miner) => !miner.attacker);
  const cursor = (state.tick * 37 + state.blocks.length * 17) % active.reduce((sum, miner) => sum + miner.hashPower, 0);
  let weight = 0;
  return active.find((miner) => {
    weight += miner.hashPower;
    return cursor < weight;
  }) ?? active[0];
}

/**
 * createPowMiners 按初始化权重生成矿工集合,最后一个矿工作为潜在攻击者。
 */
function createPowMiners(params: SimInitParams, minerCount: number, validTip: string) {
  const shares = weightedShares(params, 'hashPower', [42, 33, 25], minerCount);
  return shares.map((hashPower, index) => ({
    id: `pow-miner-${String.fromCharCode(97 + index)}`,
    label: `矿工 ${String.fromCharCode(65 + index)}`,
    hashPower,
    validTip,
    accepted: true,
    attacker: false,
  }));
}

/**
 * attackerMiner 返回当前私有分叉攻击者,未标记时使用最后一个矿工作为攻击发起者。
 */
function attackerMiner(state: PowState) {
  return state.miners.find((miner) => miner.attacker) ?? state.miners[state.miners.length - 1] ?? state.miners[0];
}

/**
 * mineCandidateWindow 枚举一个 nonce 窗口,让哈希搜索能被可视化为连续过程。
 */
function mineCandidateWindow(parent: PowBlock, minerId: string, mempoolSize: number, difficulty: number, startNonce: number, windowSize: number): { nonce: number; hash: string; attempts: PowAttempt[]; found: boolean } {
  const attempts: PowAttempt[] = [];
  for (let offset = 0; offset < windowSize; offset += 1) {
    const nonce = startNonce + offset;
    const hash = hashPowCandidate(parent.hash, parent.height + 1, minerId, nonce, mempoolSize);
    const attempt = { nonce, hash, score: leadingZeroNibbles(hash), valid: blockHashValid(hash, difficulty) };
    if (attempts.length >= 8) attempts.shift();
    attempts.push(attempt);
    if (attempt.valid) return { nonce, hash, attempts, found: true };
  }
  const last = attempts[attempts.length - 1] ?? { nonce: startNonce, hash: hashPowCandidate(parent.hash, parent.height + 1, minerId, startNonce, mempoolSize), score: 0, valid: false };
  return { nonce: last.nonce, hash: last.hash, attempts, found: false };
}

/**
 * hashPowCandidate 用父哈希、高度、矿工、nonce 和交易池生成可验证区块哈希。
 */
function hashPowCandidate(parentHash: string, height: number, minerId: string, nonce: number, mempoolSize: number): string {
  return canonicalConsensusDigest('pow-header', { height, mempoolSize, minerId, nonce, parentHash }, 16);
}

/**
 * blockValid 复算区块头并校验哈希是否满足当前目标。
 */
function blockValid(block: PowBlock): boolean {
  if (block.height === 0) {
    return true;
  }
  const recomputed = hashPowCandidate(block.parentHash, block.height, block.minerId, block.nonce, block.mempoolSize);
  return block.hash === recomputed && blockHashValid(block.hash, block.difficulty);
}

/**
 * blockHashValid 使用十六进制前导零表示教学难度目标。
 */
function blockHashValid(hash: string, difficulty: number): boolean {
  return leadingZeroNibbles(hash) >= difficulty;
}

/**
 * leadingZeroNibbles 统计哈希前导零半字节数。
 */
function leadingZeroNibbles(hash: string): number {
  let count = 0;
  for (const char of hash) {
    if (char !== '0') return count;
    count += 1;
  }
  return count;
}

/**
 * targetPrefix 把难度转换为用户可理解的目标前缀。
 */
function targetPrefix(difficulty: number): string {
  return '0'.repeat(Math.max(0, difficulty));
}

/**
 * workForDifficulty 将目标难度转换为累计工作量权重。
 */
function workForDifficulty(difficulty: number): number {
  return difficulty <= 0 ? 0 : 16 ** Math.min(6, difficulty);
}

/**
 * continueSelfishMining 按窗口推进攻击者私有分叉搜索,命中后才追加私有块。
 */
function continueSelfishMining(state: PowState): PowState {
  const parent = state.privateFork[state.privateFork.length - 1] ?? state.blocks[state.blocks.length - 1];
  const attacker = attackerMiner(state);
  const cursor = Math.max(1, state.privateMiningCursor);
  const search = mineCandidateWindow(parent, attacker.id, state.mempoolSize, state.difficulty, cursor, state.hashWindowSize);
  const candidateBase = {
    ...state,
    lastTransition: 'selfish-mining',
    selfishMining: true,
    candidateParentHash: parent.hash,
    candidateMinerId: attacker.id,
    candidateNonce: search.nonce,
    candidateHash: search.hash,
    candidateReady: search.found,
    hashAttempts: search.attempts,
    privateMiningCursor: search.nonce + 1,
    privateMiningTargetDepth: Math.max(state.privateForkTargetDepth, state.privateMiningTargetDepth),
  };
  if (!search.found) {
    return candidateBase;
  }
  const mined = createPowBlock({
    id: `pow-private-${state.privateFork.length + 1}`,
    height: parent.height + 1,
    minerId: attacker.id,
    parentHash: parent.hash,
    difficulty: state.difficulty,
    nonce: search.nonce,
    mempoolSize: state.mempoolSize,
    hash: search.hash,
    attacker: true,
    canonical: false,
  });
  return { ...candidateBase, candidateReady: false, privateFork: state.privateFork.concat(mined) };
}
