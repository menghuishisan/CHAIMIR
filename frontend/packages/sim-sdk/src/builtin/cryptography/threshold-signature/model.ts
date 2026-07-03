// 本文件定义门限签名仿真的领域模型和阶段元数据。

import type { SimState } from '../../../types';
import type { CryptoActor, CryptoMessage } from '../cryptoView';

export interface ShareHolder extends CryptoActor {
  share: string;
  x: number;
  shareValue: number;
  coefficient?: number;
  partialSignature?: string;
  signed: boolean;
  faulty: boolean;
}

export interface ThresholdState extends SimState {
  phaseIndex: number;
  threshold: number;
  messageDigest: string;
  groupPublicKey: number;
  polynomial: number[];
  aggregateSignature: string;
  holders: ShareHolder[];
  messages: CryptoMessage[];
  aggregateValid: boolean;
  lastTransition: string;
}

export const thresholdSignaturePhases = [
  { id: 'split', label: '拆分密钥份额', detail: '生成 n 个份额', effect: '私钥被拆成多个份额,单个参与方拿不到完整私钥。', reason: '门限方案把单点私钥风险拆散到多个参与方。' },
  { id: 'assign', label: '分发份额', detail: '安全交给签名者', effect: '每个签名者只保存自己的密钥份额。', reason: '份额分发后聚合者也不应该知道完整私钥。' },
  { id: 'partial-sign', label: '生成部分签名', detail: '各自签名摘要', effect: '在线签名者用本地份额生成部分签名。', reason: '部分签名不能单独作为完整签名使用。' },
  { id: 'aggregate', label: '聚合签名', detail: '收集 t 个份额', effect: '聚合者收集至少 t 个有效部分签名并组合。', reason: '达到门限才能恢复可验证的群体签名。' },
  { id: 'verify', label: '验证聚合签名', detail: '公钥检查', effect: '验证者用群公钥检查聚合签名。', reason: '外部只需要看群签名,不需要知道哪些份额参与。' },
  { id: 'exclude', label: '剔除故障份额', detail: '重新选择签名者', effect: '发现无效份额后剔除并从候补中补足门限。', reason: '门限签名需要在部分节点故障时仍保持可用。' },
] as const;
