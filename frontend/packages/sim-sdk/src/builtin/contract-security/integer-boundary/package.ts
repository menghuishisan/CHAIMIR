// 本文件装配整数边界与 checked 运算仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialIntegerState, integerSafe, reduceIntegerEvent } from './kernel';
import { integerCodeTrace, integerNarrative } from './trace';
import { renderIntegerView } from './view';
import type { IntegerBoundaryState } from './model';

export const integerBoundarySimulation: SimPackage<IntegerBoundaryState> = {
  meta: { code: 'builtin__security-integer-boundary', name: '整数边界与 checked 运算推演', category: 'contract-security', version: '1.0.0', compute: 'frontend', summary: '完整推演整数输入校验、溢出路径、精度截断、checked 运算防护和边界测试覆盖。', learningObjectives: ['理解数值边界为什么属于安全问题', '掌握 checked 运算和业务 cap', '观察边界测试如何覆盖临界输入'], scaleLimit: { nodes: 48, maxTick: 100, maxEvents: 200 } },
  initState: createInitialIntegerState,
  reducer: reduceIntegerEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择用例', description: '查看边界用例结果。', emits: 'select', target: 'element', elementFilter: 'integer-case' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进整数边界流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '输入极大值', description: '注入超出业务范围的数值。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '启用 checked', description: '启用范围限制和 checked 运算。', emits: 'recover', labelTag: 'perturb' }],
  render: renderIntegerView,
  narrative: integerNarrative,
  codeTrace: integerCodeTrace,
  checkpoints: [{ id: 'integer-boundary-safe', label: '整数边界受控', evaluate: (state) => integerSafe(state as IntegerBoundaryState) }],
};
