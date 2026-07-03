// 本文件定义跨链多签委员会仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface CommitteeMember {
  id: string;
  label: string;
  signature?: string;
  signed: boolean;
  malicious: boolean;
  active: boolean;
}

export interface CommitteeState extends SimState {
  phaseIndex: number;
  threshold: number;
  messageHash: string;
  aggregateSignature: string;
  members: CommitteeMember[];
  aggregateReady: boolean;
  authorized: boolean;
  lastTransition: string;
}

export const committeePhases = [
  { id: 'rotate', label: '委员会轮换', detail: '确定活跃成员', effect: '系统确定当前跨链消息的签名委员会。', reason: '委员会成员需要可轮换,避免长期固定信任集合。' },
  { id: 'sign', label: '成员签名', detail: '对消息摘要签名', effect: '活跃成员对跨链消息摘要签名。', reason: '多签授权要求多个独立成员确认。' },
  { id: 'aggregate', label: '聚合签名', detail: '达到门限', effect: '聚合器收集至少 threshold 个有效签名。', reason: '门限聚合降低单点私钥风险。' },
  { id: 'filter', label: '剔除恶意签名', detail: '验证签名者身份', effect: '无效或非活跃成员签名被拒绝。', reason: '只数有效成员签名,不能只数签名数量。' },
  { id: 'authorize', label: '执行授权', detail: '授权目标链执行', effect: '签名门限满足后目标链允许执行消息。', reason: '授权是多签桥的最终安全闸门。' },
];
