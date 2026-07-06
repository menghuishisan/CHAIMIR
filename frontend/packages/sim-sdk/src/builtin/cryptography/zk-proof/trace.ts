// 本文件定义零知识证明仿真的代码追踪和教学叙事。

import type { CodeTraceDef } from '../../../types';
import { phaseNarrative } from '../../packageTools';
import { zkProofPhases } from './model';

export const zkProofSource = [
  'function verifyZk(commitment, challenge, response) {',
  '  require(commitment.isBound());',
  '  expected = relation(response, challenge);',
  '  require(expected == commitment);',
  '  repeatUntil(soundnessErrorLow());',
  '}',
];

/**
 * traceLinesForZkProof 把零知识证明迁移映射到伪代码行。
 */
export function traceLinesForZkProof(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    witness: [1],
    commit: [2],
    challenge: [3],
    response: [3, 4],
    verify: [4],
    repeat: [5],
    cheat: [3, 4],
  };
  return mapping[transition] ?? [1];
}

export const zkProofCodeTrace: CodeTraceDef = {
  sourceCode: zkProofSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == commit', annotation: '承诺必须先绑定证明者的选择。' },
    { line: 3, triggerCondition: 'lastTransition == challenge || lastTransition == response || lastTransition == cheat', annotation: '响应由挑战和见证共同决定。' },
    { line: 4, triggerCondition: 'lastTransition == response || lastTransition == verify || lastTransition == cheat', annotation: '验证等式必须成立才接受证明。', highlightStyle: 'success' },
    { line: 5, triggerCondition: 'lastTransition == repeat', annotation: '多轮独立挑战降低作弊通过概率。', highlightStyle: 'success' },
  ],
  variableWatch: [
    { name: 'challenge', extract: 'state.challenge', format: 'number' },
    { name: 'response', extract: 'state.response', format: 'hex' },
    { name: 'verifierResult', extract: 'state.verifierResult', format: 'bool' },
  ],
};

export const zkProofNarrative = phaseNarrative(zkProofPhases, 'zk-proof-valid');
