// 本文件定义跨链重放防护仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface ReplayState extends SimState {
  phaseIndex: number;
  domain: string;
  nonce: number;
  messageHash: string;
  executedNonces: number[];
  replayAttempt: boolean;
  accepted: boolean;
  lastTransition: string;
}

export const replayPhases = [
  { id: 'domain', label: '写入域分离', detail: '绑定链 ID 和应用 ID', effect: '消息哈希包含源链、目标链和应用标识。', reason: '域分离防止同一签名跨链或跨应用复用。' },
  { id: 'nonce', label: '分配消息 nonce', detail: '单调序号', effect: '每条跨链消息获得唯一 nonce。', reason: 'nonce 是识别重放消息的最直接依据。' },
  { id: 'execute', label: '执行并记录', detail: '写入已执行集合', effect: '目标链执行消息后记录该 nonce。', reason: '已执行集合让重复提交能被拒绝。' },
  { id: 'replay', label: '拒绝重放消息', detail: '检测已用 nonce', effect: '相同 domain 和 nonce 的消息再次提交会失败。', reason: '签名和证明有效不代表消息可以重复执行。' },
  { id: 'rotate', label: '版本轮换', detail: '升级 domain 版本', effect: '协议升级时更新 domain 版本并保留历史 nonce 记录。', reason: '版本轮换避免升级后旧消息绕过防重放。' },
];
