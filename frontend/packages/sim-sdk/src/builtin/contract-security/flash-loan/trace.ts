// 本文件定义闪电贷组合攻击仿真的 Solidity 代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { flashLoanPhases } from './model';

export const flashLoanSource = ['function executeFlashLoan() {', '  loan = borrow(pool);', '  manipulateMarket(loan);', '  exploitProtocol();', '  repay(loan + fee);', '  require(blockImpact < limit);', '}'];
export const flashLoanNarrative = phaseNarrative(flashLoanPhases, 'flash-loan-contained');
export const flashLoanCodeTrace = { sourceCode: flashLoanSource.join('\n'), language: 'solidity' as const, lineMapping: flashLoanPhases.map((phase, index) => ({ line: Math.min(index + 1, flashLoanSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'exploit' ? ('error' as const) : phase.id === 'limit' ? ('success' as const) : ('normal' as const) })), variableWatch: [{ name: 'attackerProfit', extract: 'state.attackerProfit', format: 'number' as const }] };

/**
 * traceLinesForFlashLoan 返回当前闪电贷阶段对应的源码行。
 */
export function traceLinesForFlashLoan(transition: string): number[] {
  const index = flashLoanPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, flashLoanSource.length)];
}
