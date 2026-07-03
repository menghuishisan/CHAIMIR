// 本文件定义跨链消息生命周期仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { crossMessagePhases } from './model';

export const crossMessageSource = ['function receiveMessage(message, proof) {', '  require(verifySourceEvent(proof));', '  require(!executed[message.id]);', '  executePayload(message.payload);', '  executed[message.id] = true;', '}'];
export const crossMessageNarrative = phaseNarrative(crossMessagePhases, 'cross-message-executed');
export const crossMessageCodeTrace = { sourceCode: crossMessageSource.join('\n'), language: 'pseudocode' as const, lineMapping: crossMessagePhases.map((phase, index) => ({ line: Math.min(index + 1, crossMessageSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'execute' ? ('success' as const) : ('normal' as const) })), variableWatch: [{ name: 'messageId', extract: 'state.messageId', format: 'hex' as const }] };

/**
 * traceLinesForCrossMessage 返回当前跨链消息阶段对应的代码行。
 */
export function traceLinesForCrossMessage(transition: string): number[] {
  const index = crossMessagePhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, crossMessageSource.length)];
}
