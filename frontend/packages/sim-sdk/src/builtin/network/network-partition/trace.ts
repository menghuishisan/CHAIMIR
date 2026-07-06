// 本文件定义网络分区与恢复仿真的代码追踪和教学叙事。

import type { CodeTraceDef } from '../../../types';
import { phaseNarrative } from '../../packageTools';
import { partitionPhases } from './model';

export const partitionSource = [
  'function handlePartition(topology) {',
  '  cutSet = findCrossRegionLinks(topology);',
  '  block(cutSet);',
  '  syncWithinRegion();',
  '  heal(cutSet);',
  '  exchangeVersions();',
  '  mergeByConsensusRule();',
  '}',
];

/**
 * traceLinesForPartition 把分区恢复迁移映射到伪代码行。
 */
export function traceLinesForPartition(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    topology: [2],
    cut: [3],
    'local-sync': [4],
    heal: [5, 6],
    merge: [7],
  };
  return mapping[transition] ?? [1];
}

export const partitionCodeTrace: CodeTraceDef = {
  sourceCode: partitionSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == topology', annotation: '先识别跨区域割边。' },
    { line: 3, triggerCondition: 'lastTransition == cut', annotation: '割边被阻断后跨区消息不可达。', highlightStyle: 'error' },
    { line: 4, triggerCondition: 'lastTransition == local-sync', annotation: '两侧分区继续本地同步并产生版本差。' },
    { line: 6, triggerCondition: 'lastTransition == heal', annotation: '恢复后先交换版本和分歧证据。' },
    { line: 7, triggerCondition: 'lastTransition == merge', annotation: '按显式规则合并到统一版本。', highlightStyle: 'success' },
  ],
  variableWatch: [
    { name: 'partitionActive', extract: 'state.partitionActive', format: 'bool' },
    { name: 'versionGap', extract: 'state.metrics.versionGap', format: 'number' },
  ],
};

export const partitionNarrative = phaseNarrative(partitionPhases, 'partition-merged');
