// 本文件定义门限签名仿真的代码追踪和教学叙事。

import type { CodeTraceDef, NarrativeStep } from '../../../types';
import { thresholdSignaturePhases } from './model';

export const thresholdSignatureSource = [
  'function thresholdSign(message) {',
  '  shares = splitSecret(privateKey, n, t);',
  '  partials = signWithShares(message, shares);',
  '  require(validPartials(partials) >= t);',
  '  signature = aggregate(partials);',
  '  require(verify(groupPublicKey, message, signature));',
  '}',
];

/**
 * traceLinesForThresholdSignature 把门限签名迁移映射到伪代码行。
 */
export function traceLinesForThresholdSignature(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    split: [2],
    assign: [2],
    'partial-sign': [3],
    aggregate: [4, 5],
    verify: [6],
    exclude: [3, 4],
  };
  return mapping[transition] ?? [1];
}

export const thresholdSignatureCodeTrace: CodeTraceDef = {
  sourceCode: thresholdSignatureSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == split || lastTransition == assign', annotation: '私钥按 n 和 t 生成份额并分发。' },
    { line: 3, triggerCondition: 'lastTransition == partial-sign || lastTransition == exclude', annotation: '每个签名者只用本地份额生成部分签名。' },
    { line: 4, triggerCondition: 'lastTransition == aggregate || lastTransition == exclude', annotation: '有效部分签名数量必须达到门限。' },
    { line: 5, triggerCondition: 'lastTransition == aggregate', annotation: '聚合器组合有效部分签名。' },
    { line: 6, triggerCondition: 'lastTransition == verify', annotation: '群公钥验证聚合签名。', highlightStyle: 'success' },
  ],
  variableWatch: [
    { name: 'threshold', extract: 'state.threshold', format: 'number' },
    { name: 'validShares', extract: 'state.metrics.validShares', format: 'number' },
    { name: 'aggregateSignature', extract: 'state.aggregateSignature', format: 'hex' },
  ],
};

export const thresholdSignatureNarrative: NarrativeStep[] = thresholdSignaturePhases.map((phase, index) => ({
  id: phase.id,
  title: phase.label,
  trigger: (state) => state.phase === phase.label,
  highlight: [phase.id],
  explain: `${phase.effect} ${phase.reason}`,
  defaultDurationMs: 1200,
  question:
    index === thresholdSignaturePhases.length - 1
      ? {
          prompt: '当前有效份额是否足以形成可验证的聚合签名?',
          options: ['足够', '不足'],
          answer: '足够',
          checkpointId: 'threshold-signature-valid',
        }
      : undefined,
}));
