// 本文件定义整数边界仿真的 Solidity 代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { integerPhases } from './model';

export const integerSource = ['function mint(amount, price) {', '  require(amount <= MAX_AMOUNT);', '  total = checkedMul(amount, price);', '  require(total <= cap);', '  mint(total);', '}'];
export const integerNarrative = phaseNarrative(integerPhases, 'integer-boundary-safe');
export const integerCodeTrace = { sourceCode: integerSource.join('\n'), language: 'solidity' as const, lineMapping: integerPhases.map((phase, index) => ({ line: Math.min(index + 1, integerSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'checked' ? ('success' as const) : ('normal' as const) })), variableWatch: [{ name: 'checkedMath', extract: 'state.checkedMath', format: 'bool' as const }] };

/**
 * traceLinesForInteger 返回当前整数边界阶段对应的源码行。
 */
export function traceLinesForInteger(transition: string): number[] {
  const index = integerPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, integerSource.length)];
}
