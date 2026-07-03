// 本文件定义零知识证明交互仿真的领域模型和阶段元数据。

import type { SimState } from '../../../types';
import type { CryptoActor, CryptoMessage } from '../cryptoView';

export interface ZkState extends SimState {
  phaseIndex: number;
  secret: number;
  randomizer: number;
  publicKey: number;
  commitment: string;
  challenge: number;
  response: string;
  responseValue: number;
  verifierResult: boolean;
  cheating: boolean;
  actors: CryptoActor[];
  messages: CryptoMessage[];
  lastTransition: string;
}

export const zkProofPhases = [
  { id: 'witness', label: '持有秘密见证', detail: '证明者知道秘密', effect: '证明者持有秘密见证,但不会直接发送给验证者。', reason: '零知识要求证明知识存在,不泄露知识本身。' },
  { id: 'commit', label: '发送承诺', detail: '隐藏见证', effect: '证明者用随机数和见证生成承诺。', reason: '承诺先锁定证明者的选择,避免看到挑战后伪造。' },
  { id: 'challenge', label: '验证者挑战', detail: '发送随机挑战', effect: '验证者发送不可预测挑战。', reason: '挑战让证明者必须对真实见证作出一致响应。' },
  { id: 'response', label: '计算响应', detail: '绑定挑战与见证', effect: '证明者返回与承诺、挑战和见证一致的响应。', reason: '响应能被校验,但不会暴露原始秘密。' },
  { id: 'verify', label: '验证等式', detail: '检查约束成立', effect: '验证者检查承诺、挑战和响应是否满足关系。', reason: '约束成立说明证明者以高概率知道见证。' },
  { id: 'repeat', label: '重复降低错误率', detail: '多轮挑战', effect: '多轮独立挑战降低作弊者碰巧通过的概率。', reason: '交互式零知识靠重复把可靠性放大。' },
] as const;
