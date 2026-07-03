// 本文件实现哈希链输入规范化、摘要计算、父哈希串联、篡改检测和链式修复内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { stringArrayParam } from '../../initParams';
import { hashChainDigest } from '../cryptoPrimitives';
import { hashChainPhases, type HashChainState, type HashRecord } from './model';
import { traceLinesForHashChain } from './trace';

/**
 * createInitialHashChainState 构造四条链式记录并完成初始摘要。
 */
export function createInitialHashChainState(params: SimInitParams, _seed: number): HashChainState {
  const payloads = stringArrayParam(params, 'payloads', ['Alice 转账 5', 'Bob 质押 3', 'Carol 投票 A', 'Dave 提交证明'], 2, 24, 96);
  const records = payloads.reduce<HashRecord[]>((list, payload, index) => {
    const parentHash = index === 0 ? 'genesis' : list[index - 1].hash;
    const hash = digest(index + 1, payload, parentHash);
    return list.concat({ id: `hash-record-${index + 1}`, index: index + 1, payload, parentHash, hash, tampered: false, valid: true });
  }, []);
  return finalizeHashChainState({
    tick: 0,
    phase: hashChainPhases[0].label,
    phaseIndex: 0,
    records,
    repaired: false,
    lastTransition: 'normalize',
    explanation: explainHashChainPhase(0),
    metrics: {},
    checkpointValues: {},
  });
}

/**
 * reduceHashChainEvent 是哈希链包唯一事件入口,保持回放确定性。
 */
export function reduceHashChainEvent(state: HashChainState, event: SimEvent, _context: ReducerContext): HashChainState {
  if (event.type === 'select') return finalizeHashChainState({ ...state, selectedElementId: event.target, selectedRecordId: event.target });
  if (event.type === 'attack') return finalizeHashChainState(tamperHashRecord(state));
  if (event.type === 'recover') return finalizeHashChainState(repairHashChain(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeHashChainState(advanceHashChain(state, event));
  return state;
}

/**
 * advanceHashChain 推进哈希链教学阶段并在校验阶段刷新有效性。
 */
export function advanceHashChain(state: HashChainState, event: SimEvent): HashChainState {
  const phaseIndex = Math.min(hashChainPhases.length - 1, state.phaseIndex + 1);
  const transition = hashChainPhases[phaseIndex].id;
  const next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: transition };
  return phaseIndex >= 3 ? verifyHashChain(next) : next;
}

/**
 * hashChainValid 检查所有记录是否重新满足链式哈希关系。
 */
export function hashChainValid(state: HashChainState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.valid);
  return { achieved, answer: { invalidCount: state.metrics.invalidCount, repaired: state.checkpointValues.repaired }, explanation: achieved ? '每条记录的摘要和父哈希均校验通过。' : '仍存在摘要或父哈希不匹配。' };
}

/**
 * finalizeHashChainState 刷新指标、检查点和代码追踪。
 */
export function finalizeHashChainState(state: HashChainState): HashChainState {
  const invalidRecords = state.records.filter((record) => !record.valid);
  const firstInvalidIndex = invalidRecords[0]?.index ?? 0;
  return {
    ...state,
    phase: hashChainPhases[state.phaseIndex].label,
    explanation: explainHashChainPhase(state.phaseIndex),
    metrics: { result: invalidRecords.length === 0 ? '哈希链一致' : '发现篡改', risk: invalidRecords.length * 25, invalidCount: invalidRecords.length, firstInvalidIndex },
    checkpointValues: { valid: invalidRecords.length === 0, repaired: state.repaired },
    _trace: {
      triggeredLines: traceLinesForHashChain(state.lastTransition),
      variables: { invalidCount: invalidRecords.length, firstInvalidIndex, repaired: state.repaired },
      executionPath: `hash-chain/${state.lastTransition}`,
    },
  };
}

/**
 * tamperHashRecord 修改选中或默认记录的载荷但保留原摘要,制造可被校验阶段发现的哈希不匹配。
 */
function tamperHashRecord(state: HashChainState): HashChainState {
  const targetId = state.selectedRecordId ?? state.records[1]?.id ?? state.records[0]?.id;
  return verifyHashChain({
    ...state,
    repaired: false,
    lastTransition: 'tamper',
    records: state.records.map((record) => {
      if (record.id !== targetId) return record;
      return { ...record, payload: `${record.payload} 已改动`, tampered: true };
    }),
  });
}

/**
 * repairHashChain 从第一条无效记录开始重算后续父哈希和摘要。
 */
function repairHashChain(state: HashChainState): HashChainState {
  const firstInvalid = state.records.findIndex((record) => !record.valid);
  const startIndex = firstInvalid < 0 ? 0 : firstInvalid;
  const records: HashRecord[] = [];
  for (const [index, record] of state.records.entries()) {
    const parentHash = records.length === 0 ? 'genesis' : records[records.length - 1].hash;
    const shouldRecalculate = index >= startIndex;
    records.push({ ...record, parentHash, hash: shouldRecalculate ? digest(record.index, record.payload, parentHash) : record.hash, tampered: false, valid: true });
  }
  return { ...state, records, repaired: true, lastTransition: 'repair', phaseIndex: hashChainPhases.length - 1 };
}

/**
 * verifyHashChain 重算每条记录并标记载荷或父哈希异常。
 */
function verifyHashChain(state: HashChainState): HashChainState {
  let previousHash = 'genesis';
  const records = state.records.map((record) => {
    const expected = digest(record.index, record.payload, previousHash);
    const valid = record.hash === expected && record.parentHash === previousHash;
    previousHash = expected;
    return { ...record, valid };
  });
  return { ...state, records, lastTransition: state.lastTransition === 'tamper' ? 'tamper' : 'verify' };
}

/**
 * digest 计算绑定序号、父摘要和规范化载荷的教学稳定摘要。
 */
export function digest(index: number, payload: string, parentHash: string): string {
  return hashChainDigest(index, payload, parentHash);
}

/**
 * explainHashChainPhase 返回当前阶段说明。
 */
function explainHashChainPhase(index: number) {
  const phase = hashChainPhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
