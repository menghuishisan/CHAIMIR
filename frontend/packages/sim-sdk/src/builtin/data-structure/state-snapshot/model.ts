// 本文件定义状态快照与回滚仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface AccountState {
  id: string;
  balance: number;
  nonce: number;
  dirty: boolean;
  restored: boolean;
}

export interface SnapshotPoint {
  x: number;
  consistency: number;
  risk: number;
  cost: number;
}

export interface SnapshotState extends SimState {
  phaseIndex: number;
  accounts: AccountState[];
  snapshotRoot: string;
  currentRoot: string;
  rollbackRoot: string;
  samples: SnapshotPoint[];
  lastTransition: string;
}

export const snapshotPhases = [
  { id: 'collect', label: '收集当前状态', detail: '读取账户键值', effect: '系统读取需要快照的账户余额和 nonce。', reason: '快照必须来自同一逻辑高度的状态视图。' },
  { id: 'root', label: '计算快照根', detail: '排序后哈希', effect: '账户状态按 key 排序并计算快照根。', reason: '稳定排序让同一状态得到同一根摘要。' },
  { id: 'delta', label: '记录增量变更', detail: '标记 dirty 项', effect: '后续交易只记录被修改账户的增量。', reason: '增量记录比完整复制更节省空间。' },
  { id: 'rollback', label: '回滚到快照', detail: '恢复 dirty 项', effect: '异常发生时将被修改账户恢复到快照版本。', reason: '回滚能力让执行失败不会污染状态。' },
  { id: 'verify', label: '校验快照根', detail: '重算根摘要', effect: '恢复后重新计算根并与快照根比较。', reason: '根一致说明状态确实恢复到快照点。' },
];
