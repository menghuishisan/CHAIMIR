// 本文件定义 EVM 调用栈仿真的 Solidity 代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { callStackPhases } from './model';

export const callStackSource = ['function callA() {', '  frame.push(A);', '  ok = B.call(data);', '  require(ok);', '  frame.pop();', '  require(depth < MAX_DEPTH);', '}'];
export const callStackNarrative = phaseNarrative(callStackPhases, 'call-stack-safe');
export const callStackCodeTrace = { sourceCode: callStackSource.join('\n'), language: 'solidity' as const, lineMapping: callStackPhases.map((phase, index) => ({ line: Math.min(index + 1, callStackSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'revert' ? ('error' as const) : phase.id === 'depth' ? ('success' as const) : ('normal' as const) })), variableWatch: [{ name: 'depth', extract: 'state.metrics.depth', format: 'number' as const }] };

/**
 * traceLinesForCallStack 返回当前调用栈阶段对应的源码行。
 */
export function traceLinesForCallStack(transition: string): number[] {
  const index = callStackPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, callStackSource.length)];
}
