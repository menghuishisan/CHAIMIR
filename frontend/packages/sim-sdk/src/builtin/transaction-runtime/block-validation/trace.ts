// 本文件定义区块验证仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { blockValidationPhases } from './model';

export const blockValidationSource = ['function validateBlock(block) {', '  require(block.parent == localTip.hash);', '  require(root(block.txs) == block.txRoot);', '  require(root(receipts) == block.receiptRoot);', '  require(execute(block).stateRoot == block.stateRoot);', '  acceptOrReject(block);', '}'];
export const blockValidationNarrative = phaseNarrative(blockValidationPhases, 'block-validation-accepted');
export const blockValidationCodeTrace = { sourceCode: blockValidationSource.join('\n'), language: 'pseudocode' as const, lineMapping: blockValidationPhases.map((phase, index) => ({ line: Math.min(index + 1, blockValidationSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'reject' ? ('error' as const) : phase.id === 'state-root' ? ('success' as const) : ('normal' as const) })), variableWatch: [{ name: 'accepted', extract: 'state.accepted', format: 'bool' as const }] };

/**
 * traceLinesForBlockValidation 返回当前区块验证阶段对应的代码行。
 */
export function traceLinesForBlockValidation(transition: string): number[] {
  const index = blockValidationPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, blockValidationSource.length)];
}
