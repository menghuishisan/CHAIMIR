// 本文件定义交易生命周期仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { txLifecyclePhases } from './model';

export const txLifecycleSource = ['function submitTransaction(tx) {', '  txHash = sign(tx);', '  validateForMempool(tx);', '  includeInBlock(tx);', '  receipt = execute(tx);', '}'];
export const txLifecycleNarrative = phaseNarrative(txLifecyclePhases, 'tx-lifecycle-receipt');
export const txLifecycleCodeTrace = { sourceCode: txLifecycleSource.join('\n'), language: 'pseudocode' as const, lineMapping: txLifecyclePhases.map((phase, index) => ({ line: Math.min(index + 1, txLifecycleSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'execute' ? ('success' as const) : ('normal' as const) })), variableWatch: [{ name: 'txHash', extract: 'state.txHash', format: 'hex' as const }] };

/**
 * traceLinesForTxLifecycle 返回当前交易生命周期阶段对应的代码行。
 */
export function traceLinesForTxLifecycle(transition: string): number[] {
  const index = txLifecyclePhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, txLifecycleSource.length)];
}
