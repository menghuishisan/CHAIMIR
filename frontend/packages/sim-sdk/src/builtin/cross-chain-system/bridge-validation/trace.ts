// 本文件定义跨链桥证明验证仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { bridgePhases } from './model';

export const bridgeSource = ['function mintWithProof(proof) {', '  require(lightClient.synced());', '  require(verifyInclusion(proof));', '  require(!proof.used);', '  mint(proof.amount);', '}'];
export const bridgeNarrative = phaseNarrative(bridgePhases, 'bridge-proof-valid');
export const bridgeCodeTrace = { sourceCode: bridgeSource.join('\n'), language: 'pseudocode' as const, lineMapping: bridgePhases.map((phase, index) => ({ line: Math.min(index + 1, bridgeSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'mint' ? ('success' as const) : ('normal' as const) })), variableWatch: [{ name: 'proofHash', extract: 'state.proofHash', format: 'hex' as const }] };

/**
 * traceLinesForBridge 返回当前桥验证阶段对应的代码行。
 */
export function traceLinesForBridge(transition: string): number[] {
  const index = bridgePhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, bridgeSource.length)];
}
