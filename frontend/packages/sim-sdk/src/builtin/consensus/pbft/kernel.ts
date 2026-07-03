// 本文件实现 PBFT 协议内核,所有可视化都从这里产生的确定性协议状态派生。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { indexFromSeed, integerParam, stringParam } from '../../initParams';
import { bftQuorumThreshold, canonicalConsensusDigest, makeVoteCertificate } from '../consensusPrimitives';
import { pbftPhases, type PbftCertificate, type PbftMessage, type PbftMessageType, type PbftReplica, type PbftState, type PbftTransition, type PbftViewChange } from './model';

/**
 * createInitialPbftState 根据初始参数构造完整 PBFT 副本集合、请求摘要和水位窗口。
 */
export function createInitialPbftState(params: SimInitParams, seed: number): PbftState {
  const replicaCount = integerParam(params, 'replicaCount', 4, 4, 10);
  const f = Math.max(1, Math.floor((replicaCount - 1) / 3));
  const from = stringParam(params, 'from', 'alice', 40);
  const to = stringParam(params, 'to', 'bob', 40);
  const amount = integerParam(params, 'amount', 10, 1, 1000000);
  const sequence = integerParam(params, 'sequence', 40 + indexFromSeed(seed, 17), 1, 1000000);
  const watermarkLow = integerParam(params, 'watermarkLow', 0, 0, sequence - 1);
  const watermarkHigh = integerParam(params, 'watermarkHigh', Math.max(sequence + 20, 100), sequence, 1000000);
  const clientId = stringParam(params, 'clientId', 'pbft-client', 64);
  const operation = `transfer(${from},${to},${amount})`;
  const digest = canonicalConsensusDigest('pbft-request', { amount, from, sequence, to }, 12);
  const replicas = Array.from({ length: replicaCount }, (_, index): PbftReplica => ({
    id: `pbft-r${index + 1}`,
    label: `R${index + 1}`,
    index,
    primary: index === 0,
    faulty: false,
    watermarks: { low: watermarkLow, high: watermarkHigh },
  }));
  return finalizePbftState({
    tick: 0,
    phase: pbftPhases[0].label,
    view: 0,
    sequence,
    f,
    phaseIndex: 0,
    request: { clientId, operation, digest, resultDigest: canonicalConsensusDigest('pbft-result', { digest, status: 'ok' }, 12) },
    replicas,
    messages: [],
    certificates: [],
    viewChanges: [],
    lastTransition: 'init',
    explanation: explainPhase(0),
    metrics: {},
    checkpointValues: {},
  });
}

/**
 * reducePbftEvent 是 PBFT 包的唯一事件入口,按用户交互或回放事件进入协议迁移。
 */
export function reducePbftEvent(state: PbftState, event: SimEvent, context: ReducerContext): PbftState {
  if (event.type === 'select') return finalizePbftState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizePbftState(injectByzantinePrimary(state, context));
  if (event.type === 'recover') return finalizePbftState(performViewChange(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizePbftState(advancePbft(state));
  return state;
}

/**
 * advancePbft 按 PBFT 协议顺序推进一个过程单元,每步都会产生真实协议消息。
 */
export function advancePbft(state: PbftState): PbftState {
  const tick = state.tick + 1;
  const currentPhaseId = pbftPhases[state.phaseIndex]?.id;
  const nextIndex = state.lastTransition === currentPhaseId ? Math.min(pbftPhases.length - 1, state.phaseIndex + 1) : state.phaseIndex;
  const base = { ...state, tick, phaseIndex: nextIndex };
  const phase = pbftPhases[nextIndex].id;
  if (phase === 'client-request') return submitClientRequest(base);
  if (phase === 'pre-prepare') return broadcastPrePrepare(base);
  if (phase === 'prepare-certificate') return collectPrepareCertificate(base);
  if (phase === 'commit-certificate') return collectCommitCertificate(base);
  if (phase === 'execute-reply') return executeAndReply(base);
  return stabilizeCheckpoint(base);
}

/**
 * injectByzantinePrimary 标记当前主节点为拜占庭,并准备一个冲突摘要供预准备阶段暴露。
 */
export function injectByzantinePrimary(state: PbftState, context: ReducerContext): PbftState {
  const primary = currentPrimary(state);
  const conflictingDigest = canonicalConsensusDigest('pbft-conflict', { digest: state.request.digest, sequence: context.seq }, 12);
  const messages =
    state.phaseIndex >= 1
      ? [
          protocolMessage({
            state,
            type: 'PRE-PREPARE',
            from: primary.id,
            to: nextBackup(state, primary.id).id,
            digest: conflictingDigest,
            accepted: false,
            detail: '主节点向一个副本发送冲突摘要,副本因同序号双提议拒绝。',
          }),
        ]
      : [];
  return {
    ...state,
    tick: state.tick + 1,
    conflictingDigest,
    lastTransition: 'fault-injected',
    messages: state.messages.concat(messages),
    replicas: state.replicas.map((replica) => (replica.id === primary.id ? { ...replica, faulty: true } : replica)),
  };
}

/**
 * performViewChange 收集达到法定人数的 VIEW-CHANGE 证据并由新主节点安装 NEW-VIEW。
 */
export function performViewChange(state: PbftState): PbftState {
  const tick = state.tick + 1;
  const oldPrimary = currentPrimary(state);
  const newPrimary = state.replicas[(oldPrimary.index + 1) % state.replicas.length];
  const view = state.view + 1;
  const viewChanges = state.replicas
    .filter((replica) => replica.id !== oldPrimary.id)
    .map<PbftViewChange>((replica) => ({
      from: replica.id,
      toPrimary: newPrimary.id,
      view,
      preparedDigest: replica.preparedDigest,
      checkpointSequence: replica.stableCheckpoint ?? 0,
    }));
  const vcMessages = viewChanges.map((change) =>
    protocolMessage({
      state: { ...state, tick, view },
      type: 'VIEW-CHANGE',
      from: change.from,
      to: change.toPrimary,
      digest: change.preparedDigest ?? state.request.digest,
      accepted: true,
      detail: '副本提交已准备证书和稳定检查点,请求进入新视图。',
    })
  );
  const newViewMessage = protocolMessage({
    state: { ...state, tick: tick + 1, view },
    type: 'NEW-VIEW',
    from: newPrimary.id,
    to: 'all',
    digest: safestDigest(state, viewChanges),
    accepted: viewChanges.length >= quorum(state),
    detail: '新主节点聚合法定人数视图切换消息并继承安全摘要。',
  });
  return {
    ...state,
    tick: tick + 1,
    view,
    phaseIndex: Math.max(1, state.phaseIndex),
    conflictingDigest: undefined,
    viewChanges,
    lastTransition: 'new-view',
    messages: state.messages.concat(vcMessages, newViewMessage),
    replicas: state.replicas.map((replica) => ({
      ...replica,
      primary: replica.id === newPrimary.id,
      faulty: false,
      acceptedPrePrepare: replica.acceptedPrePrepare ?? state.request.digest,
    })),
  };
}

/**
 * pbftSafetyCheckpoint 检查 committed-local 是否满足法定人数匹配提交票。
 */
export function pbftSafetyCheckpoint(state: PbftState): CheckpointResult {
  const certificate = findCertificate(state, 'committed');
  const achieved = certificate?.achieved === true;
  return {
    achieved,
    answer: { commitSigners: certificate?.signers.length ?? 0, quorum: quorum(state), digest: state.request.digest },
    explanation: achieved ? '提交证书达到法定人数,正确副本不会为同一序号提交不同摘要。' : '提交证书尚未达到法定人数,还不能执行请求。',
  };
}

/**
 * pbftViewChangeCheckpoint 检查异常主节点是否已被法定人数视图切换消息替换。
 */
export function pbftViewChangeCheckpoint(state: PbftState): CheckpointResult {
  const achieved = state.viewChanges.length >= quorum(state) || !state.conflictingDigest;
  return {
    achieved,
    answer: { view: state.view, viewChangeMessages: state.viewChanges.length, quorum: quorum(state) },
    explanation: state.viewChanges.length >= quorum(state) ? '视图切换消息达到法定人数,新主节点已经安装安全视图。' : '当前还没有收集足够视图切换消息。',
  };
}

/**
 * pbftCheckpointStability 检查稳定检查点是否由法定人数副本确认。
 */
export function pbftCheckpointStability(state: PbftState): CheckpointResult {
  const certificate = findCertificate(state, 'checkpoint');
  const achieved = certificate?.achieved === true;
  return {
    achieved,
    answer: { checkpointSigners: certificate?.signers.length ?? 0, sequence: state.sequence },
    explanation: achieved ? '稳定检查点达到法定人数,日志可以安全截断。' : '检查点票数不足,日志仍需保留。',
  };
}

/**
 * submitClientRequest 把客户端请求送到当前主节点。
 */
function submitClientRequest(state: PbftState): PbftState {
  return {
    ...state,
    lastTransition: 'client-request',
    messages: state.messages.concat(
      protocolMessage({
        state,
        type: 'REQUEST',
        from: state.request.clientId,
        to: currentPrimary(state).id,
        digest: state.request.digest,
        accepted: true,
        detail: '客户端请求进入主节点,等待主节点为本视图分配序号。',
      })
    ),
  };
}

/**
 * broadcastPrePrepare 让主节点向所有备份副本广播摘要,并显式标记冲突摘要的拒绝路径。
 */
function broadcastPrePrepare(state: PbftState): PbftState {
  const primary = currentPrimary(state);
  const messages = backups(state).map((replica, index) => {
    const digest = state.conflictingDigest && index === 0 ? state.conflictingDigest : state.request.digest;
    const accepted = digest === state.request.digest && withinWatermark(state, replica);
    return protocolMessage({
      state,
      type: 'PRE-PREPARE',
      from: primary.id,
      to: replica.id,
      digest,
      accepted,
      detail: accepted ? '副本接受主节点的视图、序号和摘要绑定。' : '副本检测到同序号冲突摘要,拒绝该预准备。',
    });
  });
  return {
    ...state,
    lastTransition: 'pre-prepare',
    messages: state.messages.concat(messages),
    replicas: state.replicas.map((replica) => (replica.primary ? replica : { ...replica, acceptedPrePrepare: acceptedDigestForReplica(messages, replica.id) })),
  };
}

/**
 * collectPrepareCertificate 广播准备票并把达到 BFT 法定人数的匹配票固化为 prepared 证书。
 */
function collectPrepareCertificate(state: PbftState): PbftState {
  const prepareSenders = state.replicas.filter((replica) => replica.primary || replica.acceptedPrePrepare === state.request.digest);
  const messages = prepareSenders.flatMap((from) =>
    state.replicas
      .filter((to) => to.id !== from.id)
      .map((to) =>
        protocolMessage({
          state,
          type: 'PREPARE',
          from: from.id,
          to: to.id,
          digest: state.request.digest,
          accepted: true,
          detail: '副本广播已接受的请求摘要,供其他副本统计 prepared 证书。',
        })
      )
  );
  const signers = prepareSenders.map((replica) => replica.id);
  const certificate = certificateFor('prepared', state.request.digest, signers, quorum(state));
  return {
    ...state,
    lastTransition: 'prepare-certificate',
    messages: state.messages.concat(messages),
    certificates: upsertCertificate(state.certificates, certificate),
    replicas: state.replicas.map((replica) => ({ ...replica, preparedDigest: certificate.achieved ? state.request.digest : replica.preparedDigest })),
  };
}

/**
 * collectCommitCertificate 只允许已 prepared 的副本广播提交票。
 */
function collectCommitCertificate(state: PbftState): PbftState {
  const commitSenders = state.replicas.filter((replica) => replica.preparedDigest === state.request.digest);
  const messages = commitSenders.flatMap((from) =>
    state.replicas
      .filter((to) => to.id !== from.id)
      .map((to) =>
        protocolMessage({
          state,
          type: 'COMMIT',
          from: from.id,
          to: to.id,
          digest: state.request.digest,
          accepted: true,
          detail: 'prepared 副本广播提交票,推动 committed-local 成立。',
        })
      )
  );
  const signers = commitSenders.map((replica) => replica.id);
  const certificate = certificateFor('committed', state.request.digest, signers, quorum(state));
  return {
    ...state,
    lastTransition: 'commit-certificate',
    messages: state.messages.concat(messages),
    certificates: upsertCertificate(state.certificates, certificate),
    replicas: state.replicas.map((replica) => ({ ...replica, committedDigest: certificate.achieved ? state.request.digest : replica.committedDigest })),
  };
}

/**
 * executeAndReply 执行已提交请求,客户端以 f+1 个一致回复作为确认条件。
 */
function executeAndReply(state: PbftState): PbftState {
  const executors = state.replicas.filter((replica) => replica.committedDigest === state.request.digest);
  const messages = executors.map((replica) =>
    protocolMessage({
      state,
      type: 'REPLY',
      from: replica.id,
      to: state.request.clientId,
      digest: state.request.resultDigest ?? state.request.digest,
      accepted: true,
      detail: '副本执行请求并向客户端返回确定性结果。',
    })
  );
  const signers = executors.map((replica) => replica.id);
  const certificate = certificateFor('reply', state.request.resultDigest ?? state.request.digest, signers, state.f + 1);
  return {
    ...state,
    lastTransition: 'execute-reply',
    messages: state.messages.concat(messages),
    certificates: upsertCertificate(state.certificates, certificate),
    replicas: state.replicas.map((replica) => ({ ...replica, executedDigest: replica.committedDigest, repliedDigest: replica.committedDigest ? state.request.resultDigest : replica.repliedDigest })),
  };
}

/**
 * stabilizeCheckpoint 让执行副本生成检查点并达成稳定证书。
 */
function stabilizeCheckpoint(state: PbftState): PbftState {
  const checkpointSenders = state.replicas.filter((replica) => replica.executedDigest === state.request.digest);
  const messages = checkpointSenders.flatMap((from) =>
    state.replicas
      .filter((to) => to.id !== from.id)
      .map((to) =>
        protocolMessage({
          state,
          type: 'CHECKPOINT',
          from: from.id,
          to: to.id,
          digest: state.request.digest,
          accepted: true,
          detail: '副本广播执行后的状态摘要,用于稳定检查点。',
        })
      )
  );
  const signers = checkpointSenders.map((replica) => replica.id);
  const certificate = certificateFor('checkpoint', state.request.digest, signers, quorum(state));
  return {
    ...state,
    lastTransition: 'stable-checkpoint',
    messages: state.messages.concat(messages),
    certificates: upsertCertificate(state.certificates, certificate),
    replicas: state.replicas.map((replica) => ({ ...replica, stableCheckpoint: certificate.achieved ? state.sequence : replica.stableCheckpoint })),
  };
}

/**
 * finalizePbftState 统一刷新教学解释、指标、检查点和代码追踪变量。
 */
export function finalizePbftState(state: PbftState): PbftState {
  const prepared = findCertificate(state, 'prepared');
  const committed = findCertificate(state, 'committed');
  const reply = findCertificate(state, 'reply');
  const checkpoint = findCertificate(state, 'checkpoint');
  const risk = state.conflictingDigest ? 90 : state.viewChanges.length > 0 ? 25 : 8;
  const phase = pbftPhases[state.phaseIndex] ?? pbftPhases[0];
  return {
    ...state,
    phase: phase.label,
    explanation: explainPhase(state.phaseIndex),
    metrics: {
      result: checkpoint?.achieved ? '检查点稳定' : reply?.achieved ? '客户端已确认' : committed?.achieved ? '已提交' : prepared?.achieved ? '已准备' : '推进中',
      risk,
      quorum: quorum(state),
      preparedVotes: prepared?.signers.length ?? 0,
      commitVotes: committed?.signers.length ?? 0,
      replyVotes: reply?.signers.length ?? 0,
      viewChangeVotes: state.viewChanges.length,
    },
    checkpointValues: {
      prepared: prepared?.achieved ?? false,
      committed: committed?.achieved ?? false,
      clientConfirmed: reply?.achieved ?? false,
      checkpointStable: checkpoint?.achieved ?? false,
      viewChanged: state.viewChanges.length >= quorum(state),
    },
    _trace: {
      triggeredLines: traceLinesFor(state.lastTransition),
      variables: {
        view: state.view,
        sequence: state.sequence,
        digest: state.request.digest,
        quorum: quorum(state),
        preparedVotes: prepared?.signers.length ?? 0,
        commitVotes: committed?.signers.length ?? 0,
      },
      executionPath: `pbft/${state.lastTransition}`,
    },
  };
}

/**
 * protocolMessage 创建带起止 tick 的协议消息,过程动画由渲染器按 progress 连续呈现。
 */
function protocolMessage(input: {
  state: PbftState;
  type: PbftMessageType;
  from: string;
  to: string;
  digest: string;
  accepted: boolean;
  detail: string;
}): PbftMessage {
  const startTick = input.state.tick;
  const endTick = startTick + (input.to === 'all' ? 2 : 1);
  return {
    id: deterministicId('pbft-msg', { type: input.type, from: input.from, to: input.to, view: input.state.view, sequence: input.state.sequence, digest: input.digest, startTick }),
    type: input.type,
    from: input.from,
    to: input.to,
    view: input.state.view,
    sequence: input.state.sequence,
    digest: input.digest,
    startTick,
    endTick,
    accepted: input.accepted,
    detail: input.detail,
  };
}

/**
 * certificateFor 创建投票证书对象。
 */
function certificateFor(type: PbftCertificate['type'], digest: string, signers: string[], threshold: number): PbftCertificate {
  const certificate = makeVoteCertificate(`pbft-${type}`, digest, signers, threshold);
  return { type, digest, signers: certificate.signers, proofDigest: certificate.proofDigest, achieved: certificate.achieved };
}

/**
 * upsertCertificate 替换同类证书,保持状态中只有每种证书的最新版本。
 */
function upsertCertificate(certificates: PbftCertificate[], certificate: PbftCertificate): PbftCertificate[] {
  return certificates.filter((item) => item.type !== certificate.type).concat(certificate);
}

/**
 * findCertificate 查找指定类型证书。
 */
export function findCertificate(state: PbftState, type: PbftCertificate['type']): PbftCertificate | undefined {
  return state.certificates.find((certificate) => certificate.type === type);
}

/**
 * quorum 返回 PBFT 当前副本集下的 BFT 法定人数。
 */
export function quorum(state: PbftState): number {
  return bftQuorumThreshold(state.replicas.length);
}

/**
 * currentPrimary 返回当前视图主节点。
 */
export function currentPrimary(state: PbftState): PbftReplica {
  return state.replicas.find((replica) => replica.primary) ?? state.replicas[0];
}

/**
 * backups 返回非主节点副本。
 */
function backups(state: PbftState): PbftReplica[] {
  return state.replicas.filter((replica) => !replica.primary);
}

/**
 * nextBackup 为攻击注入选择一个非主节点接收冲突摘要。
 */
function nextBackup(state: PbftState, primaryId: string): PbftReplica {
  return state.replicas.find((replica) => replica.id !== primaryId) ?? state.replicas[0];
}

/**
 * withinWatermark 检查序号是否位于副本水位窗口内。
 */
function withinWatermark(state: PbftState, replica: PbftReplica): boolean {
  return state.sequence > replica.watermarks.low && state.sequence <= replica.watermarks.high;
}

/**
 * acceptedDigestForReplica 从预准备消息中取出副本接受的摘要。
 */
function acceptedDigestForReplica(messages: PbftMessage[], replicaId: string): string | undefined {
  return messages.find((message) => message.to === replicaId && message.accepted)?.digest;
}

/**
 * safestDigest 依据视图切换携带的 prepared 证书选择新视图摘要。
 */
function safestDigest(state: PbftState, viewChanges: PbftViewChange[]): string {
  const preparedDigest = viewChanges.find((change) => change.preparedDigest)?.preparedDigest;
  return preparedDigest ?? state.request.digest;
}

/**
 * explainPhase 生成面向学习者的当前过程说明。
 */
function explainPhase(index: number) {
  const phase = pbftPhases[index] ?? pbftPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}

/**
 * traceLinesFor 把协议迁移映射到代码追踪高亮行。
 */
function traceLinesFor(transition: PbftTransition): number[] {
  const mapping: Record<PbftTransition, number[]> = {
    init: [1],
    'client-request': [2, 3],
    'pre-prepare': [4, 5],
    'prepare-certificate': [6, 7],
    'commit-certificate': [8, 9],
    'execute-reply': [10, 11],
    'stable-checkpoint': [12, 13],
    'fault-injected': [14, 15],
    'view-change': [16, 17],
    'new-view': [18, 19],
  };
  return mapping[transition];
}
