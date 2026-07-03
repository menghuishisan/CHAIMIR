// 本文件定义 PoW 最长链仿真的代码追踪和叙事说明。

import type { CodeTraceDef, NarrativeStep } from '../../../types';
import { powPhases } from './model';

export const powSource = [
  'function mine(parent, txs, difficulty) {',
  '  block = assembleBlock(parent, txs);',
  '  target = prefixZeros(difficulty);',
  '  do {',
  '    nonce++;',
  '    hash = H(parent.hash, block.body, nonce);',
  '  } while (!hash.startsWith(target));',
  '  broadcast(block, nonce, hash);',
  '  require(parentKnown(block) && validateWork(hash, target));',
  '  if (work(block.chain) > work(local.chain)) switchTip(block);',
  '  difficulty = retarget(window, targetSpacing);',
  '}',
];

/**
 * traceLinesForPow 把 PoW 内核迁移映射到伪代码的精确高亮行。
 */
export function traceLinesForPow(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    mempool: [1],
    assemble: [2, 3],
    'hash-search': [4, 5, 6, 7],
    broadcast: [8],
    validate: [9],
    'longest-chain': [10],
    adjust: [11],
    'selfish-mining': [4, 5, 6, 7, 10],
  };
  return mapping[transition] ?? [1];
}

export const powCodeTrace: CodeTraceDef = {
  sourceCode: powSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == assemble', annotation: '矿工把当前最高累计工作量链尖作为父块。' },
    { line: 3, triggerCondition: 'lastTransition == assemble', annotation: '难度被转换成哈希必须满足的目标前缀。' },
    { line: 5, triggerCondition: 'lastTransition == hash-search', annotation: '内核真实枚举 nonce,每个候选哈希都可由状态复算。' },
    { line: 7, triggerCondition: 'lastTransition == hash-search', annotation: '只有哈希达到目标前缀才结束搜索。' },
    { line: 8, triggerCondition: 'lastTransition == broadcast', annotation: '有效区块携带 nonce 和哈希向网络传播。' },
    { line: 9, triggerCondition: 'lastTransition == validate', annotation: '节点独立校验父块存在和工作量目标。', highlightStyle: 'success' },
    { line: 10, triggerCondition: 'lastTransition == longest-chain || lastTransition == selfish-mining', annotation: '规范链选择比较累计工作量而非单个高度。', highlightStyle: 'success' },
    { line: 11, triggerCondition: 'lastTransition == adjust', annotation: '按最近出块窗口和目标间隔调整难度。' },
  ],
  variableWatch: [
    { name: 'difficulty', extract: 'state.difficulty', format: 'number' },
    { name: 'target', extract: 'state.targetPrefix', format: 'string' },
    { name: 'nonce', extract: 'state.candidateNonce', format: 'number' },
    { name: 'candidateHash', extract: 'state.candidateHash', format: 'hex' },
    { name: 'privateDepth', extract: 'state.privateFork.length', format: 'number' },
    { name: 'hashWindowSize', extract: 'state.hashWindowSize', format: 'number' },
  ],
};

export const powNarrative: NarrativeStep[] = powPhases.map((phase, index) => ({
  id: phase.id,
  title: phase.label,
  trigger: (state) => state.phase === phase.label,
  highlight: [phase.id],
  explain: `${phase.effect} ${phase.reason}`,
  defaultDurationMs: 1200,
  question:
    index === powPhases.length - 1
      ? {
          prompt: '当前规范链是否由最高累计工作量决定?',
          options: ['是', '否'],
          answer: '是',
          checkpointId: 'pow-fork-choice',
        }
      : undefined,
}));
