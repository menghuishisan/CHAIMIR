// 本文件定义 PoS 仿真的代码追踪和教学叙事。

import type { CodeTraceDef, NarrativeStep } from '../../../types';
import { posPhases } from './model';

export const posSource = [
  'function proofOfStake(slot) {',
  '  seed = mixRandomness(history, slot);',
  '  proposer = weightedSelect(activeValidators, seed);',
  '  committee = shuffle(activeValidators, seed, slot);',
  '  block = proposer.propose(slot);',
  '  attestations = committee.sign(source, target, block.root);',
  '  aggregate = aggregateSignatures(attestations);',
  '  require(stake(attestations) >= twoThirds(activeStake));',
  '  justify(epoch);',
  '  if (previousJustified(epoch - 1)) finalize(epoch - 1);',
  '  slash(doubleVotesOrSurroundVotes(attestations));',
  '}',
];

/**
 * traceLinesForPos 把 PoS 内核迁移映射到代码追踪高亮行。
 */
export function traceLinesForPos(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    randomness: [2],
    proposer: [3, 4],
    propose: [5],
    attest: [6, 7],
    justify: [8, 9],
    finalize: [10],
    slash: [11],
  };
  return mapping[transition] ?? [1];
}

export const posCodeTrace: CodeTraceDef = {
  sourceCode: posSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == randomness', annotation: '历史随机性和 slot 生成可复现种子。' },
    { line: 3, triggerCondition: 'lastTransition == proposer', annotation: '按活跃权益权重抽取提议者。' },
    { line: 4, triggerCondition: 'lastTransition == proposer', annotation: '委员会由种子洗牌产生,不是全体节点固定投票。' },
    { line: 5, triggerCondition: 'lastTransition == propose', annotation: '提议者广播当前 slot 的区块根。' },
    { line: 6, triggerCondition: 'lastTransition == attest', annotation: '委员会对 source、target 和区块根签名见证。' },
    { line: 7, triggerCondition: 'lastTransition == attest', annotation: '签名聚合后进入权益门槛判断。' },
    { line: 9, triggerCondition: 'lastTransition == justify', annotation: '达到三分之二活跃权益后证明检查点。', highlightStyle: 'success' },
    { line: 10, triggerCondition: 'lastTransition == finalize', annotation: '连续证明后最终确定前一检查点。', highlightStyle: 'success' },
    { line: 11, triggerCondition: 'lastTransition == slash', annotation: '双签和 surround vote 会触发罚没。', highlightStyle: 'error' },
  ],
  variableWatch: [
    { name: 'slot', extract: 'state.slot', format: 'number' },
    { name: 'epoch', extract: 'state.epoch', format: 'number' },
    { name: 'blockRoot', extract: 'state.blockRoot', format: 'hex' },
    { name: 'aggregateSignature', extract: 'state.aggregateSignature', format: 'hex' },
  ],
};

export const posNarrative: NarrativeStep[] = posPhases.map((phase, index) => ({
  id: phase.id,
  title: phase.label,
  trigger: (state) => state.phase === phase.label,
  highlight: [phase.id],
  explain: `${phase.effect} ${phase.reason}`,
  defaultDurationMs: 1200,
  question:
    index === posPhases.length - 2
      ? {
          prompt: 'PoS 检查点最终确定是否需要足够权益见证?',
          options: ['需要', '不需要'],
          answer: '需要',
          checkpointId: 'pos-two-thirds-finality',
        }
      : undefined,
}));
