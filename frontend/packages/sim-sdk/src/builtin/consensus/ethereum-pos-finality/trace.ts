// 本文件定义 Ethereum PoS 最终性仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { ethPosFinalityPhases } from './model';

export const ethPosFinalitySource = [
  'function onSlot(slot) {',
  '  block = propose(parent=head);',
  '  collectLatestAttestations(block);',
  '  head = lmdGhost(justifiedCheckpoint);',
  '  if votes(target) >= 2/3: justify(target);',
  '  if consecutiveJustified(): finalize(source);',
  '}',
];

export const ethPosFinalityNarrative = phaseNarrative(ethPosFinalityPhases, 'eth-pos-finalized');

export const ethPosFinalityCodeTrace = {
  sourceCode: ethPosFinalitySource.join('\n'),
  language: 'pseudocode' as const,
  lineMapping: ethPosFinalityPhases.map((phase, index) => ({ line: Math.min(index + 1, ethPosFinalitySource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'finalize' ? ('success' as const) : ('normal' as const) })),
  variableWatch: [
    { name: 'head', extract: 'state.head', format: 'string' as const },
    { name: 'justified', extract: 'state.justified', format: 'string' as const },
    { name: 'finalized', extract: 'state.finalized', format: 'string' as const },
  ],
};

export function traceLinesForEthPosFinality(transition: string): number[] {
  const index = ethPosFinalityPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, ethPosFinalitySource.length)];
}
