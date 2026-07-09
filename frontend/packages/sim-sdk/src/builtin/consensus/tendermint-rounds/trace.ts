// 本文件定义 Tendermint 轮次仿真的代码追踪和教学叙事。

import { phaseNarrative } from '../../packageTools';
import { tendermintRoundPhases } from './model';

export const tendermintRoundsSource = [
  'function runRound(round) {',
  '  proposal = proposer(round).broadcast();',
  '  prevotes = collectPrevotes(proposal);',
  '  if power(prevotes) >= 2/3: lock(proposal.value);',
  '  precommits = collectPrecommits(lockedValue);',
  '  if power(precommits) >= 2/3: commit(lockedValue);',
  '}',
];

export const tendermintRoundsNarrative = phaseNarrative(tendermintRoundPhases, 'tendermint-commit');

export const tendermintRoundsCodeTrace = {
  sourceCode: tendermintRoundsSource.join('\n'),
  language: 'pseudocode' as const,
  lineMapping: tendermintRoundPhases.map((phase, index) => ({ line: Math.min(index + 1, tendermintRoundsSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'commit' ? ('success' as const) : phase.id === 'timeout' ? ('error' as const) : ('normal' as const) })),
  variableWatch: [
    { name: 'round', extract: 'state.round', format: 'number' as const },
    { name: 'committedValue', extract: 'state.committedValue', format: 'string' as const },
  ],
};

/** traceLinesForTendermintRounds 根据当前阶段返回 Tendermint 轮次伪代码高亮行。 */
export function traceLinesForTendermintRounds(transition: string): number[] {
  const index = tendermintRoundPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 1, tendermintRoundsSource.length)];
}
