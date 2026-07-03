// 本文件定义跨链重放防护仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { replayPhases } from './model';

export const replaySource = ['function executeCrossMessage(message) {', '  digest = hash(domain, message.nonce, payload);', '  require(!executed[domain][nonce]);', '  executed[domain][nonce] = true;', '  execute(payload);', '}'];
export const replayNarrative = phaseNarrative(replayPhases, 'replay-protected');
export const replayCodeTrace = { sourceCode: replaySource.join('\n'), language: 'pseudocode' as const, lineMapping: replayPhases.map((phase, index) => ({ line: Math.min(index + 1, replaySource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'replay' ? ('error' as const) : phase.id === 'execute' ? ('success' as const) : ('normal' as const) })), variableWatch: [{ name: 'nonce', extract: 'state.nonce', format: 'number' as const }] };

/**
 * traceLinesForReplay 返回当前重放防护阶段对应的代码行。
 */
export function traceLinesForReplay(transition: string): number[] {
  const index = replayPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, replaySource.length)];
}
