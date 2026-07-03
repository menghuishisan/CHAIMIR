// 本文件装配门限签名仿真包,算法内核、视图和追踪分别由同目录文件维护。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { createInitialThresholdSignatureState, reduceThresholdSignatureEvent, thresholdAggregateValid } from './kernel';
import { thresholdSignatureCodeTrace, thresholdSignatureNarrative } from './trace';
import { renderThresholdSignatureView } from './view';
import type { ThresholdState } from './model';

export const thresholdSignatureSimulation: SimPackage<ThresholdState> = {
  meta: {
    code: 'builtin__crypto-threshold-signature',
    name: '门限签名聚合推演',
    category: 'cryptography',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演门限签名的密钥分片、部分签名、门限聚合、群公钥验证和故障份额剔除。',
    learningObjectives: ['理解 t-of-n 门限安全性', '掌握部分签名和聚合签名区别', '观察故障份额如何被剔除并补足门限'],
    scaleLimit: { nodes: 80, maxTick: 140, maxEvents: 220 },
  },
  initState: createInitialThresholdSignatureState,
  reducer: reduceThresholdSignatureEvent,
  interactions: commonAlgorithmInteractions('share-holder'),
  render: renderThresholdSignatureView,
  narrative: thresholdSignatureNarrative,
  codeTrace: thresholdSignatureCodeTrace,
  checkpoints: [{ id: 'threshold-signature-valid', label: '门限签名聚合有效', evaluate: (state) => thresholdAggregateValid(state as ThresholdState) }],
};
