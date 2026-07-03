// 本文件定义数字签名与重放防护仿真的领域模型和阶段元数据。

import type { SimState } from '../../../types';
import type { CryptoActor, CryptoMessage } from '../cryptoView';

export interface SignatureState extends SimState {
  phaseIndex: number;
  signerKey: string;
  verifierKey: string;
  keyRegistry: Record<string, string>;
  message: string;
  digest: string;
  signature: string;
  recoveredKey: string;
  nonce: number;
  verified: boolean;
  replayDetected: boolean;
  actors: CryptoActor[];
  messages: CryptoMessage[];
  lastTransition: string;
}

export const digitalSignaturePhases = [
  { id: 'keypair', label: '生成密钥对', detail: '建立公私钥关系', effect: '签名者持有私钥,验证者只需要可信公钥。', reason: '私钥不离开签名者,公钥用于公开验签。' },
  { id: 'digest', label: '计算消息摘要', detail: '绑定消息与 nonce', effect: '消息和 nonce 被压缩为待签名摘要。', reason: 'nonce 让同一消息不能被无限重放。' },
  { id: 'sign', label: '私钥签名', detail: '生成签名值', effect: '签名者用私钥对摘要生成签名。', reason: '只有持有私钥的一方才能生成可通过公钥验证的签名。' },
  { id: 'verify', label: '公钥验签', detail: '恢复并比较摘要', effect: '验证者用公钥、消息和签名确认来源与完整性。', reason: '验签同时确认消息未改动且签名者身份可信。' },
  { id: 'replay', label: '重放检测', detail: '检查 nonce 是否已用', effect: '系统拒绝已使用 nonce 的旧签名。', reason: '签名本身有效不代表这次请求仍然新鲜。' },
  { id: 'rotate', label: '密钥轮换', detail: '撤销旧公钥', effect: '发现泄露或重放后轮换公钥并重新签名。', reason: '密钥生命周期管理是签名系统的安全边界。' },
] as const;
