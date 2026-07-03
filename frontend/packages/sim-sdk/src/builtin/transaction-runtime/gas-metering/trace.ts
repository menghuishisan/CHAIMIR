// 本文件定义 Gas 计量仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { gasPhases } from './model';

export const gasSource = ['function executeWithGas(tx) {', '  require(gasUsed <= gasLimit);', '  gasUsed += cost(op);', '  if gasUsed > gasLimit: revert();', '  refund = cappedRefund();', '  settleFee(gasUsed - refund);', '}'];
export const gasNarrative = phaseNarrative(gasPhases, 'gas-execution-settled');
export const gasCodeTrace = { sourceCode: gasSource.join('\n'), language: 'pseudocode' as const, lineMapping: gasPhases.map((phase, index) => ({ line: Math.min(index + 1, gasSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'settle' ? ('success' as const) : phase.id === 'oog' ? ('error' as const) : ('normal' as const) })), variableWatch: [{ name: 'gasUsed', extract: 'state.gasUsed', format: 'number' as const }] };

/**
 * traceLinesForGas 返回当前 Gas 阶段对应的代码行。
 */
export function traceLinesForGas(transition: string): number[] {
  const index = gasPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, gasSource.length)];
}
