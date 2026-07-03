// 本文件装配 UTXO 集合仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialUtxoState, reduceUtxoEvent, utxoValid } from './kernel';
import { utxoCodeTrace, utxoNarrative } from './trace';
import { renderUtxoView } from './view';
import type { UtxoState } from './model';

export const utxoSetSimulation: SimPackage<UtxoState> = {
  meta: { code: 'builtin__data-utxo-set', name: 'UTXO 集合更新推演', category: 'data-structure', version: '1.0.0', compute: 'frontend', summary: '完整推演 UTXO 输入引用、未花费校验、双花检测、找零输出与集合更新。', learningObjectives: ['理解 UTXO 如何替代账户余额', '掌握双花检测依据', '观察找零输出和集合压缩'], scaleLimit: { nodes: 80, maxTick: 120, maxEvents: 220 } },
  initState: createInitialUtxoState,
  reducer: reduceUtxoEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择输出', description: '选择一个 UTXO 查看引用关系。', emits: 'select', target: 'element', elementFilter: 'utxo' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进 UTXO 交易验证。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '注入双花', description: '让交易引用已花费输出。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '更新集合', description: '标记输入已花费并加入新输出。', emits: 'recover', labelTag: 'perturb' }],
  render: renderUtxoView,
  narrative: utxoNarrative,
  codeTrace: utxoCodeTrace,
  checkpoints: [{ id: 'utxo-set-valid', label: 'UTXO 交易有效', evaluate: (state) => utxoValid(state as UtxoState) }],
};
