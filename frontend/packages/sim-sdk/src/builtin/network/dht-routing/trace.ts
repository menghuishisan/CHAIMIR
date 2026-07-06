// 本文件定义 DHT 异或路由仿真的代码追踪和教学叙事。

import type { CodeTraceDef } from '../../../types';
import { phaseNarrative } from '../../packageTools';
import { dhtPhases } from './model';

export const dhtSource = [
  'function dhtLookup(key) {',
  '  shortlist = nearestFromBuckets(key);',
  '  shortlist = sortByXorDistance(shortlist, key);',
  '  while hasUnqueriedCloser(shortlist):',
  '    replies = query(alphaClosestUnqueried);',
  '    shortlist = mergeAndSortByXor(replies);',
  '  return closestValue();',
  '}',
];

/**
 * traceLinesForDht 把 DHT 内核迁移映射到伪代码行。
 */
export function traceLinesForDht(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    'id-space': [1],
    bucket: [2],
    distance: [3],
    query: [4, 5, 6],
    repair: [6, 7],
    pollute: [5, 6],
  };
  return mapping[transition] ?? [1];
}

export const dhtCodeTrace: CodeTraceDef = {
  sourceCode: dhtSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == bucket', annotation: '从 K 桶取出初始候选短名单。' },
    { line: 3, triggerCondition: 'lastTransition == distance', annotation: '候选节点按 XOR 距离排序。' },
    { line: 5, triggerCondition: 'lastTransition == query || lastTransition == pollute', annotation: '每轮只查询 alpha 个最近且未查询候选。' },
    { line: 6, triggerCondition: 'lastTransition == query || lastTransition == repair', annotation: '回复节点合并回短名单并重新排序。' },
    { line: 7, triggerCondition: 'lastTransition == repair', annotation: '污染候选剔除后返回最近可用值。', highlightStyle: 'success' },
  ],
  variableWatch: [
    { name: 'lookupKey', extract: 'state.lookupKey', format: 'number' },
    { name: 'hops', extract: 'state.hops', format: 'number' },
    { name: 'shortlistSize', extract: 'state.metrics.shortlistSize', format: 'number' },
    { name: 'bucketSize', extract: 'state.bucketSize', format: 'number' },
  ],
};

export const dhtNarrative = phaseNarrative(dhtPhases, 'dht-lookup-found');
