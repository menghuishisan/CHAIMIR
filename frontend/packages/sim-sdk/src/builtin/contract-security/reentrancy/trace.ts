// 本文件定义重入攻击仿真的 Solidity 代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { reentrancyPhases } from './model';

export const reentrancySource = ['function withdraw() {', '  require(balance[msg.sender] > 0);', '  amount = balance[msg.sender];', '  sendValue(msg.sender, amount);', '  balance[msg.sender] = 0;', '  nonReentrant();', '}'];

export const reentrancyNarrative = phaseNarrative(reentrancyPhases, 'reentrancy-blocked');

export const reentrancyCodeTrace = {
  sourceCode: reentrancySource.join('\n'),
  language: 'solidity' as const,
  lineMapping: reentrancyPhases.map((phase, index) => ({ line: Math.min(index + 2, reentrancySource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'callback' ? ('error' as const) : phase.id === 'guard' ? ('success' as const) : ('normal' as const) })),
  variableWatch: [{ name: 'vaultBalance', extract: 'state.vaultBalance', format: 'number' as const }],
};

/**
 * traceLinesForReentrancy 返回当前重入阶段对应的源码行。
 */
export function traceLinesForReentrancy(transition: string): number[] {
  const index = reentrancyPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 2, reentrancySource.length)];
}
