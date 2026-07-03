// 本文件定义 Nonce 顺序仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { noncePhases } from './model';

export const nonceSource = ['function includeAccountTxs(txs) {', '  require(tx.nonce == account.nonce);', '  execute(tx);', '  account.nonce += 1;', '  replaceByHigherFee(sameNonceTx);', '}'];
export const nonceNarrative = phaseNarrative(noncePhases, 'nonce-order-valid');
export const nonceCodeTrace = { sourceCode: nonceSource.join('\n'), language: 'pseudocode' as const, lineMapping: noncePhases.map((phase, index) => ({ line: Math.min(index + 1, nonceSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'include' ? ('success' as const) : ('normal' as const) })), variableWatch: [{ name: 'accountNonce', extract: 'state.accountNonce', format: 'number' as const }] };

/**
 * traceLinesForNonce 返回当前 nonce 阶段对应的代码行。
 */
export function traceLinesForNonce(transition: string): number[] {
  const index = noncePhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, nonceSource.length)];
}
