// 本文件定义 Mempool 替换交易与 nonce 队列仿真的状态模型。

import type { SimState } from '../../../types';
import type { RuntimeMessage } from '../runtimeView';

export type PoolStatus = 'pending' | 'queued' | 'replaced' | 'rejected' | 'included';

export interface PoolTx {
  id: string;
  account: string;
  nonce: number;
  fee: number;
  status: PoolStatus;
  reason: string;
}

export interface PoolNodeView {
  node: string;
  seen: string[];
}

export interface MempoolReplacementState extends SimState {
  phaseIndex: number;
  expectedNonce: Record<string, number>;
  transactions: PoolTx[];
  nodeViews: PoolNodeView[];
  messages: RuntimeMessage[];
  replacementRequiredBump: number;
  lastTransition: string;
}

export const mempoolReplacementPhases = [
  { id: 'receive', label: '接收交易', detail: '按账户和 nonce 入池', effect: '节点先把交易按账户 nonce 分组。', reason: '同一账户交易必须按 nonce 顺序执行。' },
  { id: 'queue', label: '划分 pending/queued', detail: '缺口交易进入队列', effect: 'nonce 正好等于 expectedNonce 的交易进入 pending,更高 nonce 等待前序交易。', reason: 'queued 交易不能越过缺失 nonce。' },
  { id: 'replace', label: '检查替换规则', detail: '同 nonce 需要足额加价', effect: '同账户同 nonce 的新交易只有满足加价阈值才替换旧交易。', reason: '替换规则防止低成本反复刷写 mempool。' },
  { id: 'propagate', label: '传播节点视图', detail: '节点间短暂不一致', effect: '不同节点看到的替换交易可能存在延迟差异。', reason: 'mempool 是本地视图,不是共识状态。' },
  { id: 'include', label: '区块打包', detail: '只打包连续 pending', effect: '构建器按连续 nonce 打包交易,打包后释放后续 queued。', reason: '链上执行顺序最终由 nonce 连续性约束。' },
  { id: 'release', label: '释放后续队列', detail: 'expectedNonce 前移', effect: '前序交易入块后,队列中下一笔变为 pending。', reason: '交易池状态随链上 nonce 前移而变化。' },
] as const;
