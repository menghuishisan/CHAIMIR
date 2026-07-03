// 本文件定义跨链最终性确认仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { finalityPhases } from './model';

export const finalitySource = ['function waitFinality(event) {', '  confirmations = head - event.height;', '  require(confirmations >= required);', '  require(!reorged(event.blockHash));', '  release(event.message);', '}'];
export const finalityNarrative = phaseNarrative(finalityPhases, 'finality-release-safe');
export const finalityCodeTrace = { sourceCode: finalitySource.join('\n'), language: 'pseudocode' as const, lineMapping: finalityPhases.map((phase, index) => ({ line: Math.min(index + 1, finalitySource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'release' ? ('success' as const) : phase.id === 'reorg' ? ('error' as const) : ('normal' as const) })), variableWatch: [{ name: 'confirmations', extract: 'state.confirmations', format: 'number' as const }] };

/**
 * traceLinesForFinality 返回当前最终性阶段对应的代码行。
 */
export function traceLinesForFinality(transition: string): number[] {
  const index = finalityPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, finalitySource.length)];
}
