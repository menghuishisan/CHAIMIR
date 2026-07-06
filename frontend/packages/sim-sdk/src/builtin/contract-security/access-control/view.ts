// 本文件把授权缺陷状态转换为权限图、权限矩阵和流程可视化。

import type { MatrixCell, TeachingFrame } from '../../../types';
import { teachingFrame, graphPattern, matrixPattern, pipelinePattern, selectedOrFrameFocus } from '../../packageTools';
import { graphEdges, graphNodes, matrixCells, pipelineSteps } from '../securityView';
import { accessPhases, type AccessState } from './model';

/**
 * renderAccessView 基于内核状态生成授权缺陷可视化。
 */
export function renderAccessView(state: AccessState): TeachingFrame {
  const exposedActors = state.actors.filter((actor) => actor.id !== 'admin' && !state.protectedFunction).length;
    const summary = `敏感函数${state.protectedFunction ? '已保护' : '未保护'},越权面 ${exposedActors} 个账户,越权${state.unauthorizedExecuted ? '已发生' : '未发生'},审计${state.auditLogged ? '已记录' : '未记录'}。`;
  const patterns = [graphPattern('access-graph', '账户角色到敏感函数的调用边界', graphNodes(state.actors), graphEdges(state.calls)), matrixPattern('access-matrix', 'RBAC 权限矩阵与审计闭环', state.actors.map((actor) => actor.label), ['角色', '敏感函数可调用', '审计记录'], accessCells(state)), pipelinePattern('access-pipeline', '授权检查 -> 执行 -> 审计流程', pipelineSteps(accessPhases, state.phaseIndex, state.unauthorizedExecuted), accessPhases[state.phaseIndex].id)];
  return teachingFrame({
    summary,
    phase: {
      id: state.phase,
      title: state.explanation.title,
      intent: 'observe',
      what: state.explanation.effect,
      why: state.explanation.reason,
      watch: summary,
    },
    focus: {
      primary: selectedOrFrameFocus(state.selectedElementId, ['access-graph']),
      secondary: ['access-matrix', 'access-pipeline'],
    },
    layout: {
      primary: 'access-graph',
      evidence: ['access-matrix', 'access-pipeline'],
    },
    patterns,
  });
}

/**
 * accessCells 展示角色、调用权限和审计状态。
 */
function accessCells(state: AccessState): MatrixCell[][] {
  return matrixCells(state.actors.map((actor) => actor.label), ['角色', '可调用', '审计'], (row, column) => {
    const actor = state.actors.find((item) => item.label === row);
    if (!actor) return { label: '无', status: 'empty' };
    if (column === '角色') return { label: actor.value ?? '', status: 'yes' };
    if (column === '敏感函数可调用') return { label: actor.id === 'admin' || !state.protectedFunction ? '是' : '否', status: actor.id === 'user' && !state.protectedFunction ? 'fault' : 'yes' };
    return { label: state.auditLogged ? '已记' : '未记', status: state.auditLogged ? 'yes' : 'pending' };
  });
}
