// 本文件定义 Optimistic Rollup 欺诈证明仿真的状态模型。

import type { SimState } from '../../../types';

export interface RollupTx {
  id: string;
  action: string;
  valid: boolean;
}

export interface DisputeSegment {
  id: string;
  fromStep: number;
  toStep: number;
  status: 'open' | 'split' | 'resolved';
}

export interface OptimisticRollupState extends SimState {
  phaseIndex: number;
  l1Height: number;
  batchId: string;
  oldRoot: string;
  claimedRoot: string;
  expectedRoot: string;
  challengeWindow: number;
  transactions: RollupTx[];
  disputeSegments: DisputeSegment[];
  challenged: boolean;
  fraudProven: boolean;
  finalized: boolean;
  lastTransition: string;
}

export const optimisticRollupPhases = [
  { id: 'sequence', label: 'Sequencer 聚合交易', detail: '排序 L2 交易', effect: 'Sequencer 把 L2 交易排序成一个 batch。', reason: 'Rollup 的吞吐来自批量提交,但排序者可能提交错误结果。' },
  { id: 'submit', label: '提交 L1 状态根', detail: '发布 batch 和 root', effect: 'batch 的 claimed state root 被提交到 L1。', reason: 'Optimistic Rollup 先乐观接受,再给挑战者留出窗口。' },
  { id: 'challenge', label: '开启挑战窗口', detail: '等待欺诈证明', effect: '挑战者检查状态转换,发现 claimedRoot 与 expectedRoot 不一致。', reason: '安全性来自任何人都能挑战错误状态。' },
  { id: 'bisect', label: '交互式二分', detail: '定位争议步骤', effect: '双方不断二分执行 trace,缩小到单个争议步骤。', reason: '二分让 L1 不必重放整个 batch。' },
  { id: 'prove', label: 'L1 单步证明', detail: '验证争议步骤', effect: 'L1 只执行最终争议步骤并判断是否欺诈。', reason: '单步证明是 optimistic fraud proof 的链上裁决点。' },
  { id: 'verdict', label: '裁决与回滚', detail: '惩罚或最终确认', effect: '欺诈成立则回滚 batch 并惩罚提交者,否则 batch 最终确认。', reason: '挑战窗口结束后的状态才能被安全依赖。' },
] as const;
