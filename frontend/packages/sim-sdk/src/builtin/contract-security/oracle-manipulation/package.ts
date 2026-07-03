// 本文件装配预言机操纵防护仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialOracleState, oracleSafe, reduceOracleEvent } from './kernel';
import { oracleCodeTrace, oracleNarrative } from './trace';
import { renderOracleView } from './view';
import type { OracleState } from './model';

export const oracleManipulationSimulation: SimPackage<OracleState> = {
  meta: { code: 'builtin__security-oracle-manipulation', name: '预言机操纵防护推演', category: 'contract-security', version: '1.0.0', compute: 'frontend', summary: '完整推演预言机现货取价、低流动性操纵、错误借款、TWAP 校验和多源聚合修复。', learningObjectives: ['理解现货价为什么不适合作为唯一预言机', '掌握 TWAP 与偏离阈值', '观察多源聚合如何降低操纵风险'], scaleLimit: { nodes: 64, maxTick: 120, maxEvents: 220 } },
  initState: createInitialOracleState,
  reducer: reduceOracleEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择价格源', description: '查看价格源当前状态。', emits: 'select', target: 'element', elementFilter: 'security-actor' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进预言机风险流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '操纵现货价', description: '用大额交易推偏现货价格。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '启用多源聚合', description: '使用 TWAP 和多源中位数修复。', emits: 'recover', labelTag: 'perturb' }],
  render: renderOracleView,
  narrative: oracleNarrative,
  codeTrace: oracleCodeTrace,
  checkpoints: [{ id: 'oracle-price-safe', label: '预言机价格受控', evaluate: (state) => oracleSafe(state as OracleState) }],
};
