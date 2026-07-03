// 本文件定义 HotStuff 链式 BFT 仿真的领域模型。

import type { SimState } from '../../../types';
import type { ViewMessage } from '../consensusView';

export interface HotStuffReplica {
  id: string;
  label: string;
  leader: boolean;
  voted: boolean;
  lockedBlock: string;
  timeout: boolean;
  faulty: boolean;
}

export interface HotStuffBlock {
  id: string;
  parentId?: string;
  view: number;
  hash: string;
  qc: boolean;
  qcSigners?: string[];
  qcDigest?: string;
  committed: boolean;
  proposerId: string;
}

export interface HotStuffState extends SimState {
  phaseIndex: number;
  view: number;
  leaderId: string;
  highQcBlock: string;
  proposalId: string;
  lockedBlock: string;
  committedBlock?: string;
  replicas: HotStuffReplica[];
  blocks: HotStuffBlock[];
  votes: Record<string, string>;
  messages: ViewMessage[];
  timeoutActive: boolean;
  lastTransition: string;
}

export const hotstuffPhases = [
  { id: 'new-view', label: '进入新视图', detail: '收集 NewView', effect: '副本把 High QC 发送给当前视图领导者。', reason: '领导者必须基于 High QC 扩展安全分支。' },
  { id: 'proposal', label: '领导者提案', detail: '扩展 High QC', effect: '领导者创建新区块并附带 High QC 广播给副本。', reason: 'High QC 约束提案只能沿已被多数认可的链延伸。' },
  { id: 'vote', label: '副本投票', detail: '校验锁与父 QC', effect: '副本验证提案是否扩展锁定块或携带更高 QC,满足后签名投票。', reason: '锁规则防止正确副本为冲突分支形成 QC。' },
  { id: 'qc', label: '形成 Quorum Certificate', detail: '聚合法定人数投票', effect: '领导者收集达到 BFT 法定人数的签名形成新区块 QC。', reason: 'QC 是 HotStuff 的安全证书,驱动下一视图和提交判断。' },
  { id: 'chain-commit', label: '三链提交', detail: '检查连续 QC', effect: '当祖父、父、子形成连续 QC 三链时提交祖父块。', reason: '链式提交把 prepare/pre-commit/commit 合并为流水线。' },
  { id: 'pacemaker', label: '超时换主', detail: '推进视图', effect: '超时副本发送 timeout,下一领导者继承 High QC。', reason: 'Pacemaker 在领导者失效时恢复活性。' },
] as const;
