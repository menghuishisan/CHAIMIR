// 本文件定义 UTXO 集合仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface Utxo {
  id: string;
  owner: string;
  amount: number;
  spent: boolean;
  selected: boolean;
  doubleSpend: boolean;
}

export interface UtxoState extends SimState {
  phaseIndex: number;
  utxos: Utxo[];
  inputs: string[];
  outputs: Utxo[];
  txValid: boolean;
  lastTransition: string;
}

export const utxoPhases = [
  { id: 'select', label: '选择输入 UTXO', detail: '引用未花费输出', effect: '交易输入引用现有未花费输出。', reason: 'UTXO 模型不改余额,只消费旧输出并创建新输出。' },
  { id: 'check', label: '检查未花费状态', detail: '拒绝已花费输入', effect: '验证器确认每个输入仍在未花费集合中。', reason: '双花检测的核心是输入不能已被消费。' },
  { id: 'sum', label: '校验输入输出金额', detail: '金额守恒', effect: '输入总额必须覆盖输出和手续费。', reason: '金额守恒防止凭空增发。' },
  { id: 'change', label: '生成找零输出', detail: '拆分新输出', effect: '多余输入金额返回给付款方作为找零。', reason: 'UTXO 输出不可部分消费,只能整体消费再重新拆分。' },
  { id: 'compact', label: '更新 UTXO 集合', detail: '删除旧输出加入新输出', effect: '已消费输出移除,新输出加入集合。', reason: '集合更新后后续交易只能引用新未花费输出。' },
];
