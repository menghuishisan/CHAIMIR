// 本文件定义状态快照与回滚仿真的代码追踪和叙事配置。

import { phaseNarrative } from '../../packageTools';
import { snapshotPhases } from './model';

export const snapshotSource = [
  'function snapshot(state) {',
  '  root = hashSortedAccounts(state);',
  '  deltas = recordDirtyWrites();',
  '  if executionFails: rollback(deltas);',
  '  require(hashSortedAccounts(state) == root);',
  '}',
];

export const snapshotNarrative = phaseNarrative(snapshotPhases, 'snapshot-root-valid');

export const snapshotCodeTrace = {
  sourceCode: snapshotSource.join('\n'),
  language: 'pseudocode' as const,
  lineMapping: snapshotPhases.map((phase, index) => ({ line: Math.min(index + 2, snapshotSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'verify' ? ('success' as const) : ('normal' as const) })),
  variableWatch: [{ name: 'snapshotRoot', extract: 'state.snapshotRoot', format: 'hex' as const }],
};

/**
 * traceLinesForSnapshot 返回当前快照阶段对应的代码行。
 */
export function traceLinesForSnapshot(transition: string): number[] {
  const index = snapshotPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 2, snapshotSource.length)];
}
