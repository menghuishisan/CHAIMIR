// 本文件定义预言机操纵仿真的 Solidity 代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { oraclePhases } from './model';

export const oracleSource = ['function priceGuard() {', '  spot = amm.getSpotPrice();', '  require(abs(spot - twap) < maxDeviation);', '  price = median(oracles);', '  borrowAgainst(price);', '}'];
export const oracleNarrative = phaseNarrative(oraclePhases, 'oracle-price-safe');
export const oracleCodeTrace = { sourceCode: oracleSource.join('\n'), language: 'solidity' as const, lineMapping: oraclePhases.map((phase, index) => ({ line: Math.min(index + 1, oracleSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'borrow' ? ('error' as const) : phase.id === 'aggregate' ? ('success' as const) : ('normal' as const) })), variableWatch: [{ name: 'spotPrice', extract: 'state.spotPrice', format: 'number' as const }] };

/**
 * traceLinesForOracle 返回当前预言机阶段对应的源码行。
 */
export function traceLinesForOracle(transition: string): number[] {
  const index = oraclePhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, oracleSource.length)];
}
