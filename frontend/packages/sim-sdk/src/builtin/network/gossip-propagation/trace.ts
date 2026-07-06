// 本文件定义 Gossip 传播仿真的代码追踪和教学叙事。

import type { CodeTraceDef } from '../../../types';
import { phaseNarrative } from '../../packageTools';
import { gossipPhases } from './model';

export const gossipSource = [
  'function gossip(message) {',
  '  frontier = seedPeers(message);',
  '  targets = chooseFanout(frontier, fanout);',
  '  deliverToUnseen(targets);',
  '  ignoreDuplicate(message.id);',
  '  require(coverage() >= target);',
  '}',
];

/**
 * traceLinesForGossip 把 Gossip 迁移映射到伪代码行。
 */
export function traceLinesForGossip(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    seed: [2],
    fanout: [3],
    spread: [3, 4],
    dedupe: [5],
    converge: [6],
    pollute: [4, 5],
  };
  return mapping[transition] ?? [1];
}

export const gossipCodeTrace: CodeTraceDef = {
  sourceCode: gossipSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == seed', annotation: '传播从少量种子节点开始。' },
    { line: 3, triggerCondition: 'lastTransition == fanout || lastTransition == spread', annotation: '每轮只向 fanout 个邻居转发。' },
    { line: 4, triggerCondition: 'lastTransition == spread || lastTransition == pollute', annotation: '只有未见过消息的节点会进入下一轮 frontier。' },
    { line: 5, triggerCondition: 'lastTransition == dedupe || lastTransition == pollute', annotation: '重复消息会计数并丢弃。' },
    { line: 6, triggerCondition: 'lastTransition == converge', annotation: '覆盖率达标后传播收敛。', highlightStyle: 'success' },
  ],
  variableWatch: [
    { name: 'coverage', extract: 'state.metrics.coverage', format: 'number' },
    { name: 'fanout', extract: 'state.fanout', format: 'number' },
    { name: 'round', extract: 'state.round', format: 'number' },
  ],
};

export const gossipNarrative = phaseNarrative(gossipPhases, 'gossip-coverage');
