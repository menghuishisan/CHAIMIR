// 本文件定义区块验证仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface ValidationItem {
  id: string;
  label: string;
  expected: string;
  actual: string;
  valid: boolean;
}

export interface BlockValidationState extends SimState {
  phaseIndex: number;
  blockHash: string;
  items: ValidationItem[];
  accepted: boolean;
  lastTransition: string;
}

export const blockValidationPhases = [
  { id: 'header', label: '校验区块头', detail: '父哈希和高度', effect: '节点检查父哈希、高度、时间和出块者。', reason: '区块头必须连接到本地已知链。' },
  { id: 'tx-root', label: '校验交易根', detail: '重建交易 Merkle 根', effect: '节点用区块交易列表重算交易根。', reason: '交易根防止区块体被替换。' },
  { id: 'receipt-root', label: '校验收据根', detail: '重建执行结果', effect: '节点检查交易执行产生的收据根。', reason: '收据根承诺事件、状态和 gas 结果。' },
  { id: 'state-root', label: '校验状态根', detail: '执行后比对', effect: '节点执行所有交易并比对状态根。', reason: '状态根是区块有效性的最终结果。' },
  { id: 'reject', label: '拒绝无效区块', detail: '不接入规范链', effect: '任一根不匹配时节点拒绝该区块。', reason: '本地验证让节点不必信任出块者。' },
];
