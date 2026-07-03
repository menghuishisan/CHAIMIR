// 本文件定义 UTXO 集合仿真的代码追踪和叙事配置。

import { phaseNarrative } from '../../packageTools';
import { utxoPhases } from './model';

export const utxoSource = [
  'function validateUtxoTx(tx) {',
  '  require(allInputsUnspent(tx.inputs));',
  '  require(sum(inputs) >= sum(outputs) + fee);',
  '  markSpent(tx.inputs);',
  '  addOutputs(tx.outputs);',
  '}',
];

export const utxoNarrative = phaseNarrative(utxoPhases, 'utxo-set-valid');

export const utxoCodeTrace = {
  sourceCode: utxoSource.join('\n'),
  language: 'pseudocode' as const,
  lineMapping: utxoPhases.map((phase, index) => ({ line: Math.min(index + 2, utxoSource.length), triggerCondition: phase.id, annotation: phase.reason, highlightStyle: phase.id === 'compact' ? ('success' as const) : ('normal' as const) })),
  variableWatch: [{ name: 'inputSum', extract: 'state.metrics.inputSum', format: 'number' as const }],
};

/**
 * traceLinesForUtxo 返回当前 UTXO 阶段对应的代码行。
 */
export function traceLinesForUtxo(transition: string): number[] {
  const index = utxoPhases.findIndex((phase) => phase.id === transition);
  return [Math.min((index < 0 ? 0 : index) + 2, utxoSource.length)];
}
