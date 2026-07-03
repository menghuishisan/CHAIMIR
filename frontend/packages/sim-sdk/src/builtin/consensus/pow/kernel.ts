// 本文件实现 PoW 最长链共识内核,渲染层只能消费这里产出的确定性状态。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { canonicalConsensusDigest } from '../consensusPrimitives';
import { processViewMessage, refreshViewMessages, type ViewMessage } from '../consensusView';
import { powPhases, type PowAttempt, type PowBlock, type PowState } from './model';
import { traceLinesForPow } from './trace';

/**
 * createInitialPowState 构造三矿工 PoW 场景,包含创世块和空私有分叉。
 */
export function createInitialPowState(_params: SimInitParams, _seed: number): PowState {
  const genesis = createPowBlock({ id: 'pow-genesis', height: 0, minerId: 'network', parentHash: 'genesis', difficulty: 0, nonce: 0, hash: '0000000000000000', attacker: false, canonical: true });
  return finalizePowState({
    tick: 0,
    phase: powPhases[0].label,
    phaseIndex: 0,
    difficulty: 4,
    targetPrefix: targetPrefix(4),
    mempoolSize: 42,
    candidateNonce: 0,
    candidateHash: hashPowCandidate(genesis.hash, 1, 'pow-miner-a', 0, 42),
    candidateParentHash: genesis.hash,
    candidateMinerId: 'pow-miner-a',
    hashAttempts: [],
    targetSpacing: 6,
    miners: [
      { id: 'pow-miner-a', label: '矿工 A', hashPower: 42, validTip: genesis.hash, accepted: true, attacker: false },
      { id: 'pow-miner-b', label: '矿工 B', hashPower: 33, validTip: genesis.hash, accepted: true, attacker: false },
      { id: 'pow-miner-c', label: '矿工 C', hashPower: 25, validTip: genesis.hash, accepted: true, attacker: false },
    ],
    blocks: [genesis],
    privateFork: [],
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
  return { achieved, answer: { canonicalWork: chainWork(state.blocks), forkWork: chainWork(state.privateFork) }, explanation: achieved ? '节点选择了累计工作量最高的链。' : '私有分叉工作量更高,需要处理重组。' };
}

/**
 * finalizePowState 刷新指标、检查点、历史样本和代码追踪。
 */
export function finalizePowState(state: PowState): PowState {
  const risk = state.selfishMining ? 72 : state.privateFork.length > 0 ? 48 : 14;
  const finality = Math.min(96, state.blocks.length * 18 - (state.selfishMining ? 20 : 0));
  const samples = state.samples.concat({ x: state.tick + state.phaseIndex, quorum: Math.min(100, chainWork(state.blocks) * 8), risk, finality }).slice(-24);
  const nonGenesisBlocks = state.blocks.filter((item) => item.height > 0);
  return {
    ...state,
    phase: powPhases[state.phaseIndex].label,
    explanation: explainPowPhase(state.phaseIndex),
    messages: refreshViewMessages(state.messages, state.tick, (message) => message.detail ?? `${message.label} 从矿工传播到对端节点。`),
    samples,
    metrics: { result: state.selfishMining ? '存在私有分叉' : '按累计工作量收敛', risk, finality, work: chainWork(state.blocks), difficulty: state.difficulty },
    checkpointValues: { workValid: nonGenesisBlocks.every((item) => blockMeetsDifficulty(item, state.difficulty)), forkChoice: chainWork(state.blocks) >= chainWork(state.privateFork) },
    _trace: { triggeredLines: traceLinesForPow(state.lastTransition), variables: { difficulty: state.difficulty, target: state.targetPrefix, nonce: state.candidateNonce, candidateHash: state.candidateHash, attempts: state.hashAttempts.length }, executionPath: `pow/${state.lastTransition}` },
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
  return { ...state, lastTransition: 'assemble', candidateParentHash: parent.hash, candidateMinerId: miner.id, candidateNonce, candidateHash, hashAttempts: [{ nonce: candidateNonce, hash: candidateHash, score: leadingZeroNibbles(candidateHash), valid: blockHashValid(candidateHash, state.difficulty) }] };
}

/**
 * searchNonce 模拟 nonce 枚举,直到当前教学难度下产生有效哈希。
 */
function searchNonce(state: PowState): PowState {
  const parent = state.blocks[state.blocks.length - 1];
  const search = mineCandidate(parent, state.candidateMinerId, state.mempoolSize, state.difficulty, state.candidateNonce);
  return { ...state, lastTransition: 'hash-search', candidateNonce: search.nonce, candidateHash: search.hash, hashAttempts: search.attempts };
}

/**
 * broadcastBlock 将有效新区块追加到规范链并广播给所有矿工。
 */
function broadcastBlock(state: PowState): PowState {
  const parent = state.blocks[state.blocks.length - 1];
  const mined = createPowBlock({ id: `pow-block-${state.blocks.length}`, height: parent.height + 1, minerId: state.candidateMinerId, parentHash: parent.hash, difficulty: state.difficulty, nonce: state.candidateNonce, hash: state.candidateHash, attacker: false, canonical: true });
  return { ...state, lastTransition: 'broadcast', blocks: state.blocks.concat(mined), messages: state.messages.concat(broadcast(state, state.candidateMinerId, '新区块')) };
}

/**
 * validateBlock 让每个矿工独立验证区块并更新本地链尖。
 */
function validateBlock(state: PowState): PowState {
  const tip = state.blocks[state.blocks.length - 1];
  const parentKnown = state.blocks.some((block) => block.hash === tip.parentHash) || tip.height === 0;
  const validWork = blockMeetsDifficulty(tip, state.difficulty);
  return { ...state, lastTransition: 'validate', miners: state.miners.map((miner) => ({ ...miner, validTip: parentKnown && validWork ? tip.hash : miner.validTip, accepted: parentKnown && validWork })) };
}

/**
 * chooseLongestChain 比较规范链与私有分叉的累计工作量并选择高工作量链。
 */
function chooseLongestChain(state: PowState): PowState {
  if (chainWork(state.privateFork) > chainWork(state.blocks)) {
    return { ...state, lastTransition: 'longest-chain', blocks: state.privateFork.map((item) => ({ ...item, canonical: true })), privateFork: [], selfishMining: false };
  }
  return { ...state, lastTransition: 'longest-chain' };
}

/**
 * adjustDifficulty 根据区块增长速度调整教学难度目标。
 */
function adjustDifficulty(state: PowState): PowState {
  const window = state.blocks.filter((block) => block.height > 0).slice(-4);
  const observedSpacing = window.length > 1 ? Math.max(1, state.tick / window.length) : state.targetSpacing;
  const nextDifficulty = observedSpacing < state.targetSpacing * 0.75 ? Math.min(4, state.difficulty + 1) : observedSpacing > state.targetSpacing * 1.5 ? Math.max(2, state.difficulty - 1) : state.difficulty;
  return { ...state, lastTransition: 'adjust', difficulty: nextDifficulty, targetPrefix: targetPrefix(nextDifficulty) };
}

/**
 * startSelfishMining 创建攻击者私有分叉但暂不发布。
 */
function startSelfishMining(state: PowState): PowState {
  const parent = state.blocks[state.blocks.length - 1];
  const first = mineCandidate(parent, 'pow-miner-c', state.mempoolSize, state.difficulty, state.candidateNonce + 31);
  const privateOne = createPowBlock({ id: 'pow-private-1', height: parent.height + 1, minerId: 'pow-miner-c', parentHash: parent.hash, difficulty: state.difficulty, nonce: first.nonce, hash: first.hash, attacker: true, canonical: false });
  const secondSearch = mineCandidate(privateOne, 'pow-miner-c', state.mempoolSize, state.difficulty, first.nonce + 31);
  const privateTwo = createPowBlock({ id: 'pow-private-2', height: parent.height + 2, minerId: 'pow-miner-c', parentHash: privateOne.hash, difficulty: state.difficulty, nonce: secondSearch.nonce, hash: secondSearch.hash, attacker: true, canonical: false });
  const fork = [privateOne, privateTwo];
  return { ...state, tick: state.tick + 1, lastTransition: 'selfish-mining', selfishMining: true, privateFork: fork, miners: state.miners.map((miner) => (miner.id === 'pow-miner-c' ? { ...miner, attacker: true } : miner)) };
}

/**
 * publishPrivateFork 发布私有分叉,让最长链规则显式处理重组。
 */
function publishPrivateFork(state: PowState): PowState {
  return chooseLongestChain({ ...state, tick: state.tick + 1, messages: state.messages.concat(broadcast(state, 'pow-miner-c', '发布私有分叉')) });
}

/**
 * createPowBlock 创建确定性 PoW 区块。
 */
export function createPowBlock(input: { id: string; height: number; minerId: string; parentHash: string; difficulty: number; nonce: number; hash: string; attacker: boolean; canonical: boolean }): PowBlock {
  return { id: input.id, height: input.height, minerId: input.minerId, parentHash: input.parentHash, hash: input.hash, nonce: input.nonce, work: workForDifficulty(input.difficulty), canonical: input.canonical, attacker: input.attacker };
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
 * mineCandidate 枚举 nonce 直到哈希满足目标前缀,并保留最近尝试供过程化可视化展示。
 */
function mineCandidate(parent: PowBlock, minerId: string, mempoolSize: number, difficulty: number, startNonce: number): { nonce: number; hash: string; attempts: PowAttempt[] } {
  const attempts: PowAttempt[] = [];
  for (let offset = 0; offset < 500000; offset += 1) {
    const nonce = startNonce + offset;
    const hash = hashPowCandidate(parent.hash, parent.height + 1, minerId, nonce, mempoolSize);
    const attempt = { nonce, hash, score: leadingZeroNibbles(hash), valid: blockHashValid(hash, difficulty) };
    if (attempts.length >= 8) attempts.shift();
    attempts.push(attempt);
    if (attempt.valid) return { nonce, hash, attempts };
  }
  const last = attempts[attempts.length - 1] ?? { nonce: startNonce, hash: hashPowCandidate(parent.hash, parent.height + 1, minerId, startNonce, mempoolSize), score: 0, valid: false };
  return { nonce: last.nonce, hash: last.hash, attempts };
}

/**
 * hashPowCandidate 用父哈希、高度、矿工、nonce 和交易池生成可验证区块哈希。
 */
function hashPowCandidate(parentHash: string, height: number, minerId: string, nonce: number, mempoolSize: number): string {
  return canonicalConsensusDigest('pow-header', { height, mempoolSize, minerId, nonce, parentHash }, 16);
}

/**
 * blockMeetsDifficulty 校验区块哈希是否满足当前目标。
 */
function blockMeetsDifficulty(block: PowBlock, difficulty: number): boolean {
  return blockHashValid(block.hash, difficulty);
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
