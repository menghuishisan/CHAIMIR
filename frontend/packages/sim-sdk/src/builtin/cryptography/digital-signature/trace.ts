// 本文件定义数字签名仿真的代码追踪和教学叙事。

import type { CodeTraceDef, NarrativeStep } from '../../../types';
import { digitalSignaturePhases } from './model';

export const digitalSignatureSource = [
  'function verifySignedMessage(message, nonce, signature) {',
  '  digest = H(message, nonce);',
  '  signer = recover(digest, signature);',
  '  require(signer == trustedPublicKey);',
  '  require(!usedNonce[nonce]);',
  '  usedNonce[nonce] = true;',
  '}',
];

/**
 * traceLinesForDigitalSignature 把签名内核迁移映射到伪代码行。
 */
export function traceLinesForDigitalSignature(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    keypair: [1],
    digest: [2],
    sign: [2, 3],
    verify: [3, 4],
    replay: [5],
    rotate: [2, 3, 6],
  };
  return mapping[transition] ?? [1];
}

export const digitalSignatureCodeTrace: CodeTraceDef = {
  sourceCode: digitalSignatureSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == digest || lastTransition == sign || lastTransition == rotate', annotation: '消息和 nonce 被绑定到同一个摘要。' },
    { line: 3, triggerCondition: 'lastTransition == sign || lastTransition == verify || lastTransition == rotate', annotation: '验签从摘要和签名恢复签名者身份。' },
    { line: 4, triggerCondition: 'lastTransition == verify', annotation: '恢复出的公钥必须等于可信公钥。', highlightStyle: 'success' },
    { line: 5, triggerCondition: 'lastTransition == replay', annotation: '有效旧签名也会因为 nonce 已使用被拒绝。', highlightStyle: 'error' },
    { line: 6, triggerCondition: 'lastTransition == rotate', annotation: '接受后记录 nonce,轮换后使用新签名。', highlightStyle: 'success' },
  ],
  variableWatch: [
    { name: 'digest', extract: 'state.digest', format: 'hex' },
    { name: 'nonce', extract: 'state.nonce', format: 'number' },
    { name: 'verified', extract: 'state.verified', format: 'bool' },
    { name: 'replayDetected', extract: 'state.replayDetected', format: 'bool' },
  ],
};

export const digitalSignatureNarrative: NarrativeStep[] = digitalSignaturePhases.map((phase, index) => ({
  id: phase.id,
  title: phase.label,
  trigger: (state) => state.phase === phase.label,
  highlight: [phase.id],
  explain: `${phase.effect} ${phase.reason}`,
  defaultDurationMs: 1200,
  question:
    index === digitalSignaturePhases.length - 1
      ? {
          prompt: '当前签名是否同时满足来源可信和 nonce 新鲜?',
          options: ['满足', '不满足'],
          answer: '满足',
          checkpointId: 'signature-valid',
        }
      : undefined,
}));
