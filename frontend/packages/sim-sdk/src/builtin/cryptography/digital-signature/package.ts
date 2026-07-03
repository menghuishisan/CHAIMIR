// 本文件装配数字签名仿真包,算法内核、视图和追踪分别由同目录文件维护。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { createInitialDigitalSignatureState, reduceDigitalSignatureEvent, signatureValid } from './kernel';
import { digitalSignatureCodeTrace, digitalSignatureNarrative } from './trace';
import { renderDigitalSignatureView } from './view';
import type { SignatureState } from './model';

export const digitalSignatureSimulation: SimPackage<SignatureState> = {
  meta: {
    code: 'builtin__crypto-digital-signature',
    name: '数字签名与重放防护推演',
    category: 'cryptography',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演数字签名的密钥生成、消息摘要、私钥签名、公钥验签、nonce 重放检测与密钥轮换。',
    learningObjectives: ['理解签名如何证明来源', '区分完整性校验和新鲜性校验', '掌握密钥轮换的必要性'],
    scaleLimit: { nodes: 48, maxTick: 120, maxEvents: 200 },
  },
  initState: createInitialDigitalSignatureState,
  reducer: reduceDigitalSignatureEvent,
  interactions: commonAlgorithmInteractions('crypto-actor'),
  render: renderDigitalSignatureView,
  narrative: digitalSignatureNarrative,
  codeTrace: digitalSignatureCodeTrace,
  checkpoints: [{ id: 'signature-valid', label: '签名验签与重放防护通过', evaluate: (state) => signatureValid(state as SignatureState) }],
};
