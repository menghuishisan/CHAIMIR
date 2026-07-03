// 本文件定义跨链多签委员会仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { committeePhases } from './model';

export const committeeSource = ['function authorize(message, sigs) {', '  valid = filterActiveSigners(sigs);', '  require(valid.length >= threshold);', '  aggregate(valid);', '  execute(message);', '}'];
export const committeeNarrative = phaseNarrative(committeePhases, 'committee-authorized');
export const committeeCodeTrace = { sourceCode: committeeSource.join('\n'), language: 'pseudocode' as const, lineMapping: committeePhases.map((phase, index) => ({ line: Math.min(index + 1, committeeSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'authorize' ? ('success' as const) : ('normal' as const) })), variableWatch: [{ name: 'validSignatures', extract: 'state.metrics.validSignatures', format: 'number' as const }] };

/**
 * traceLinesForCommittee 返回当前委员会阶段对应的代码行。
 */
export function traceLinesForCommittee(transition: string): number[] {
  const index = committeePhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, committeeSource.length)];
}
