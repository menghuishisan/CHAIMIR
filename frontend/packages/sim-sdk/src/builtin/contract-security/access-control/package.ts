// 本文件装配授权缺陷与最小权限仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { accessSafe, createInitialAccessState, reduceAccessEvent } from './kernel';
import { accessCodeTrace, accessNarrative } from './trace';
import { renderAccessView } from './view';
import type { AccessState } from './model';

export const accessControlSimulation: SimPackage<AccessState> = {
  meta: { code: 'builtin__security-access-control', name: '授权缺陷与最小权限推演', category: 'contract-security', version: '1.0.0', compute: 'frontend', summary: '完整推演角色声明、敏感函数鉴权、越权执行、审计记录和最小权限修复。', learningObjectives: ['理解敏感操作为什么必须服务端或链上鉴权', '识别缺少 onlyRole 的越权路径', '掌握最小权限和审计的防护价值'], scaleLimit: { nodes: 64, maxTick: 120, maxEvents: 220 } },
  initState: createInitialAccessState,
  reducer: reduceAccessEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择账户', description: '查看账户角色和调用结果。', emits: 'select', target: 'element', elementFilter: 'security-actor' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进授权校验流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '尝试越权', description: '让普通用户调用敏感函数。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '应用最小权限', description: '为敏感函数启用角色校验。', emits: 'recover', labelTag: 'perturb' }],
  render: renderAccessView,
  narrative: accessNarrative,
  codeTrace: accessCodeTrace,
  checkpoints: [{ id: 'access-control-safe', label: '敏感操作已受控', evaluate: (state) => accessSafe(state as AccessState) }],
};
