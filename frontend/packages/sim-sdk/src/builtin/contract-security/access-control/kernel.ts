// 本文件实现授权缺陷的角色声明、鉴权、越权调用、审计和最小权限修复内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { processSecurityCall, type SecurityActor, type SecurityCall } from '../securityView';
import { accessPhases, type AccessState } from './model';
import { traceLinesForAccess } from './trace';

/**
 * createInitialAccessState 创建管理员、普通用户和配置合约。
 */
export function createInitialAccessState(_params: SimInitParams, _seed: number): AccessState {
  const actors: SecurityActor[] = [{ id: 'admin', label: '管理员', role: 'security-actor', status: 'active', value: 'ADMIN' }, { id: 'user', label: '普通用户', role: 'security-actor', status: 'idle', value: 'USER' }, { id: 'config', label: '配置合约', role: 'security-actor', status: 'idle', value: '敏感函数' }];
  return finalizeAccessState({ tick: 0, phase: accessPhases[0].label, phaseIndex: 0, roles: { admin: 'ADMIN', user: 'USER' }, protectedFunction: false, unauthorizedExecuted: false, auditLogged: false, actors, calls: [], lastTransition: 'roles', explanation: explain(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceAccessEvent 是授权缺陷仿真的唯一事件入口。
 */
export function reduceAccessEvent(state: AccessState, event: SimEvent, _context: ReducerContext): AccessState {
  if (event.type === 'select') return finalizeAccessState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeAccessState(exploit(state));
  if (event.type === 'recover') return finalizeAccessState(repair(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeAccessState(advanceAccess(state, event));
  return state;
}

/**
 * advanceAccess 按授权防护流程推进一个过程单元。
 */
export function advanceAccess(state: AccessState, event: SimEvent): AccessState {
  const phaseIndex = Math.min(accessPhases.length - 1, state.phaseIndex + 1);
  let next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: accessPhases[phaseIndex].id };
  if (phaseIndex === 2) next = exploit(next);
  if (phaseIndex === 3) next = { ...next, auditLogged: true };
  if (phaseIndex === 4) next = repair(next);
  return next;
}

/**
 * finalizeAccessState 刷新权限状态、指标、检查点和代码追踪。
 */
export function finalizeAccessState(state: AccessState): AccessState {
  const safe = state.protectedFunction && !state.unauthorizedExecuted && state.auditLogged;
  return { ...state, phase: accessPhases[state.phaseIndex].label, actors: state.actors.map((actor) => ({ ...actor, status: actor.id === 'user' && state.unauthorizedExecuted ? 'danger' : actor.id === 'config' && safe ? 'success' : actor.status })), explanation: explain(state.phaseIndex), metrics: { result: safe ? '权限已受控' : state.unauthorizedExecuted ? '发生越权' : '等待修复', risk: safe ? 8 : state.unauthorizedExecuted ? 86 : 30 }, checkpointValues: { safe }, _trace: { triggeredLines: traceLinesForAccess(state.lastTransition), variables: { protectedFunction: state.protectedFunction, auditLogged: state.auditLogged }, executionPath: `access-control/${state.lastTransition}` } };
}

/**
 * accessSafe 输出授权安全检查点。
 */
export function accessSafe(state: AccessState): CheckpointResult {
  return { achieved: Boolean(state.checkpointValues.safe), answer: { protectedFunction: state.protectedFunction, auditLogged: state.auditLogged }, explanation: state.checkpointValues.safe ? '敏感函数已受角色保护且审计可追踪。' : '敏感函数仍存在越权或审计缺口。' };
}

/**
 * exploit 模拟普通用户越权调用。
 */
function exploit(state: AccessState): AccessState {
  return { ...state, phaseIndex: 2, lastTransition: 'exploit', unauthorizedExecuted: !state.protectedFunction, calls: state.calls.concat(call('user', 'config', 'setConfig', state.tick, '普通用户调用未受保护的敏感函数。')) };
}

/**
 * repair 启用角色校验并补齐审计记录。
 */
function repair(state: AccessState): AccessState {
  return { ...state, phaseIndex: 4, lastTransition: 'least', protectedFunction: true, unauthorizedExecuted: false, auditLogged: true };
}

/**
 * call 创建带过程跨度的授权调用。
 */
function call(from: string, to: string, label: string, at: number, detail: string): SecurityCall {
  return processSecurityCall({ id: deterministicId('access-call', { from, to, label, at }), from, to, label, at, status: 'delivered' }, detail);
}

/**
 * explain 生成阶段说明。
 */
function explain(index: number) {
  const phase = accessPhases[index] ?? accessPhases[0];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
