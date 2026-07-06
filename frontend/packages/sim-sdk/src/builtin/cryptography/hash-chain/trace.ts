// 本文件定义哈希链仿真的代码追踪和教学叙事。

import type { CodeTraceDef } from '../../../types';
import { phaseNarrative } from '../../packageTools';
import { hashChainPhases } from './model';

export const hashChainSource = [
  'function verifyHashChain(records) {',
  '  bytes = canonicalSerialize(record.payload);',
  '  hash = H(bytes, record.parentHash);',
  '  require(hash == record.hash);',
  '  require(record.parentHash == previous.hash);',
  '  repairFrom(firstInvalidIndex);',
  '}',
];

/**
 * traceLinesForHashChain 把哈希链内核迁移映射到伪代码行。
 */
export function traceLinesForHashChain(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    normalize: [2],
    hash: [3],
    link: [4, 5],
    verify: [3, 4, 5],
    repair: [6],
    tamper: [4, 5],
  };
  return mapping[transition] ?? [1];
}

export const hashChainCodeTrace: CodeTraceDef = {
  sourceCode: hashChainSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == normalize', annotation: '先把输入规范化为唯一字节序列。' },
    { line: 3, triggerCondition: 'lastTransition == hash || lastTransition == verify', annotation: '摘要由规范化输入和父哈希共同决定。' },
    { line: 4, triggerCondition: 'lastTransition == link || lastTransition == verify || lastTransition == tamper', annotation: '存储摘要必须等于重算摘要。' },
    { line: 5, triggerCondition: 'lastTransition == link || lastTransition == verify || lastTransition == tamper', annotation: '父哈希必须等于前一条记录摘要。' },
    { line: 6, triggerCondition: 'lastTransition == repair', annotation: '修复从第一条异常记录开始向后重算。', highlightStyle: 'success' },
  ],
  variableWatch: [
    { name: 'invalidCount', extract: 'state.metrics.invalidCount', format: 'number' },
    { name: 'firstInvalidIndex', extract: 'state.metrics.firstInvalidIndex', format: 'number' },
    { name: 'repaired', extract: 'state.checkpointValues.repaired', format: 'bool' },
  ],
};

export const hashChainNarrative = phaseNarrative(hashChainPhases, 'hash-chain-valid');
