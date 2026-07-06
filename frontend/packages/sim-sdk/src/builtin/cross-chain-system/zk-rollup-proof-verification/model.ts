// 本文件定义 ZK Rollup 批次证明与 L1 验证仿真的状态模型。

import type { SimState } from '../../../types';

export interface ZkBatchInput {
  id: string;
  kind: 'tx' | 'public-input' | 'proof';
  value: string;
  valid: boolean;
}

export interface ZkRollupState extends SimState {
  phaseIndex: number;
  batchId: string;
  oldRoot: string;
  newRoot: string;
  publicInputRoot: string;
  proofGenerated: boolean;
  proofValid: boolean;
  verifierAccepted: boolean;
  batchSize: number;
  provingTime: number;
  inputs: ZkBatchInput[];
  history: Array<{ x: number; provingTime: number; batchSize: number; l1Gas: number }>;
  lastTransition: string;
}

export const zkRollupPhases = [
  { id: 'aggregate', label: '聚合 L2 交易', detail: '生成 batch', effect: 'Sequencer 把多笔 L2 交易聚合为一个 batch。', reason: 'ZK Rollup 通过批量证明把大量执行压缩成一次 L1 验证。' },
  { id: 'trace', label: '生成执行 trace', detail: '计算 witness', effect: '执行交易并生成证明系统需要的 witness 和 public inputs。', reason: '证明必须绑定 oldRoot、newRoot 和公开输入。' },
  { id: 'prove', label: '生成 validity proof', detail: 'Prover 生成证明', effect: 'Prover 对执行 trace 生成有效性证明。', reason: '有效性证明让 L1 不必重放所有 L2 交易。' },
  { id: 'verify', label: 'L1 Verifier 校验', detail: '校验 proof 和 public inputs', effect: 'L1 verifier 检查 proof 是否与公开输入和状态根一致。', reason: '只有 verifier 通过的新状态根才能被 L1 接受。' },
  { id: 'update', label: '更新 L1 状态承诺', detail: '写入 newRoot', effect: '验证通过后,L1 更新 rollup 的状态根承诺。', reason: '状态根更新是用户提款和跨链证明的信任锚。' },
  { id: 'reject', label: '拒绝错误证明', detail: '保持旧状态根', effect: 'proof 或 public input 不匹配时,batch 不生效。', reason: 'ZK Rollup 的安全性来自错误状态不能通过验证。' },
] as const;
