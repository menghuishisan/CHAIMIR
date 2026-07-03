// 本文件实现现货价读取、低流动性操纵、TWAP 校验和多源聚合修复内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { processSecurityCall, type SecurityActor, type SecurityCall } from '../securityView';
import { oraclePhases, type OracleState } from './model';
import { traceLinesForOracle } from './trace';

/**
 * createInitialOracleState 创建预言机参与方和价格基线。
 */
export function createInitialOracleState(_params: SimInitParams, _seed: number): OracleState {
  const actors: SecurityActor[] = [{ id: 'amm', label: 'AMM 池', role: 'security-actor', status: 'active' }, { id: 'lending', label: '借贷合约', role: 'security-actor', status: 'idle' }, { id: 'attacker', label: '攻击者', role: 'security-actor', status: 'idle' }];
  return finalizeOracleState({ tick: 0, phase: oraclePhases[0].label, phaseIndex: 0, spotPrice: 100, twapPrice: 100, referencePrice: 100, manipulationActive: false, actors, calls: [], lastTransition: 'read', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceOracleEvent 是预言机操纵仿真的唯一事件入口。
 */
export function reduceOracleEvent(state: OracleState, event: SimEvent, _context: ReducerContext): OracleState {
  if (event.type === 'select') return finalizeOracleState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeOracleState(manipulate(state));
  if (event.type === 'recover') return finalizeOracleState(aggregate(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeOracleState(advanceOracle(state, event));
  return state;
}

/**
 * advanceOracle 按价格风险流程推进一个过程单元。
 */
export function advanceOracle(state: OracleState, event: SimEvent): OracleState {
  const phaseIndex = Math.min(oraclePhases.length - 1, state.phaseIndex + 1);
  let next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: oraclePhases[phaseIndex].id };
  if (phaseIndex === 1 || phaseIndex === 2) next = manipulate(next);
  if (phaseIndex === 4) next = aggregate(next);
  return next;
}

/**
 * finalizeOracleState 刷新价格风险、检查点和代码追踪。
 */
export function finalizeOracleState(state: OracleState): OracleState {
  const deviation = Math.abs(state.spotPrice - state.referencePrice);
  const safe = deviation <= 5 && !state.manipulationActive;
  return { ...state, phase: oraclePhases[state.phaseIndex].label, actors: state.actors.map((actor) => ({ ...actor, status: actor.id === 'attacker' && state.manipulationActive ? 'danger' : actor.id === 'lending' && safe ? 'success' : actor.status })), explanation: explain(state.phaseIndex), metrics: { result: safe ? '价格受控' : '价格偏离', risk: safe ? 8 : 84, deviation }, checkpointValues: { safe }, _trace: { triggeredLines: traceLinesForOracle(state.lastTransition), variables: { spotPrice: state.spotPrice, twapPrice: state.twapPrice }, executionPath: `oracle/${state.lastTransition}` } };
}

/**
 * oracleSafe 输出预言机检查点。
 */
export function oracleSafe(state: OracleState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.safe), answer: { spotPrice: state.spotPrice, twapPrice: state.twapPrice }, explanation: state.checkpointValues.safe ? '价格源偏离在阈值内。' : '价格仍存在可利用偏离。' };
}

/**
 * manipulate 推偏现货价格并记录价格调用。
 */
function manipulate(state: OracleState): OracleState {
  return { ...state, lastTransition: state.lastTransition === 'read' ? 'swap' : state.lastTransition, spotPrice: 168, manipulationActive: true, calls: state.calls.concat(call('attacker', 'amm', '大额兑换', state.tick, '攻击者用低流动性池推偏现货价。'), call('amm', 'lending', '偏移价格', state.tick, '借贷合约读取被操纵的现货价。')) };
}

/**
 * aggregate 启用 TWAP 与多源聚合恢复可信价格。
 */
function aggregate(state: OracleState): OracleState {
  return { ...state, phaseIndex: 4, lastTransition: 'aggregate', spotPrice: 102, twapPrice: 101, manipulationActive: false };
}

/**
 * call 创建带过程跨度的价格调用。
 */
function call(from: string, to: string, label: string, at: number, detail: string): SecurityCall {
  return processSecurityCall({ id: deterministicId('oracle-call', { from, to, label, at }), from, to, label, at, status: 'delivered' }, detail);
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = oraclePhases[index] ?? oraclePhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
