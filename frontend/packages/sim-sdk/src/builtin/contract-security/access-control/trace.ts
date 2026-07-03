// 本文件定义授权缺陷仿真的 Solidity 代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { accessPhases } from './model';

export const accessSource = ['function setConfig(value) {', '  require(hasRole(ADMIN, msg.sender));', '  config = value;', '  emit Audit(msg.sender, "setConfig");', '}'];

export const accessNarrative = phaseNarrative(accessPhases, 'access-control-safe');

export const accessCodeTrace = {
  sourceCode: accessSource.join('\n'),
  language: 'solidity' as const,
  lineMapping: accessPhases.map((phase, index) => ({ line: Math.min(index + 1, accessSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'exploit' ? ('error' as const) : phase.id === 'least' ? ('success' as const) : ('normal' as const) })),
  variableWatch: [{ name: 'protectedFunction', extract: 'state.protectedFunction', format: 'bool' as const }],
};

/**
 * traceLinesForAccess 返回当前授权阶段对应的源码行。
 */
export function traceLinesForAccess(transition: string): number[] {
  const index = accessPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, accessSource.length)];
}
