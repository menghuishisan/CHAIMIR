// 本文件定义 PBFT 内置仿真包的协议模型,只描述状态和消息结构,不承载状态迁移逻辑。

import type { SimState } from '../../../types';

export type PbftMessageType = 'REQUEST' | 'PRE-PREPARE' | 'PREPARE' | 'COMMIT' | 'REPLY' | 'CHECKPOINT' | 'VIEW-CHANGE' | 'NEW-VIEW';
export type PbftTransition =
  | 'init'
  | 'client-request'
  | 'pre-prepare'
  | 'prepare-certificate'
  | 'commit-certificate'
  | 'execute-reply'
  | 'stable-checkpoint'
  | 'fault-injected'
  | 'view-change'
  | 'new-view';

export interface PbftPhaseDef {
  id: PbftTransition;
  label: string;
  effect: string;
  reason: string;
  detail: string;
}

export interface PbftReplica {
  id: string;
  label: string;
  index: number;
  primary: boolean;
  faulty: boolean;
  acceptedPrePrepare?: string;
  preparedDigest?: string;
  committedDigest?: string;
  executedDigest?: string;
  repliedDigest?: string;
  stableCheckpoint?: number;
  watermarks: {
    low: number;
    high: number;
  };
}

export interface PbftRequest {
  clientId: string;
  operation: string;
  digest: string;
  resultDigest?: string;
}

export interface PbftMessage {
  id: string;
  type: PbftMessageType;
  from: string;
  to: string;
  view: number;
  sequence: number;
  digest: string;
  startTick: number;
  endTick: number;
  accepted: boolean;
  detail: string;
}

export interface PbftCertificate {
  type: 'prepared' | 'committed' | 'checkpoint' | 'reply';
  digest: string;
  signers: string[];
  proofDigest: string;
  achieved: boolean;
}

export interface PbftViewChange {
  from: string;
  toPrimary: string;
  view: number;
  preparedDigest?: string;
  checkpointSequence: number;
}

export interface PbftState extends SimState {
  view: number;
  sequence: number;
  f: number;
  phaseIndex: number;
  request: PbftRequest;
  replicas: PbftReplica[];
  messages: PbftMessage[];
  certificates: PbftCertificate[];
  viewChanges: PbftViewChange[];
  conflictingDigest?: string;
  lastTransition: PbftTransition;
}

export const pbftPhases: PbftPhaseDef[] = [
  {
    id: 'client-request',
    label: '客户端请求',
    effect: '客户端把操作发送给当前视图主节点,主节点为请求绑定视图、序号和摘要。',
    reason: '后续所有投票都必须围绕同一个 view、sequence、digest 三元组展开。',
    detail: '请求进入主节点日志',
  },
  {
    id: 'pre-prepare',
    label: '预准备广播',
    effect: '主节点广播 PRE-PREPARE,副本校验水位、视图、序号和摘要冲突。',
    reason: '正确副本在同一视图同一序号只接受一个摘要,这是 PBFT 安全性的第一道约束。',
    detail: '主节点广播摘要',
  },
  {
    id: 'prepare-certificate',
    label: '准备证书',
    effect: '已接受预准备的副本广播 PREPARE,并统计达到 BFT 法定人数的匹配摘要票。',
    reason: 'prepared 证书证明足够副本已经看见同一请求摘要,可阻断单个拜占庭主节点的双提议。',
    detail: '收集准备票',
  },
  {
    id: 'commit-certificate',
    label: '提交证书',
    effect: 'prepared 副本广播 COMMIT,达到 BFT 法定人数后进入 committed-local。',
    reason: '提交证书把局部可执行性扩展为跨正确副本的一致提交承诺。',
    detail: '收集提交票',
  },
  {
    id: 'execute-reply',
    label: '执行回复',
    effect: 'committed 副本执行请求并向客户端返回结果,客户端等待 f+1 个一致回复。',
    reason: 'f+1 个一致回复保证至少有一个正确副本执行了该结果。',
    detail: '执行并回复客户端',
  },
  {
    id: 'stable-checkpoint',
    label: '稳定检查点',
    effect: '副本对已执行序号广播 CHECKPOINT,达到 BFT 法定人数后稳定检查点。',
    reason: '稳定检查点用于日志截断,也为后续视图切换提供安全历史。',
    detail: '稳定检查点',
  },
];
