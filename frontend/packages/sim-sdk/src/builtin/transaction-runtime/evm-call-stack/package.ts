// 本文件装配 EVM 调用栈与 revert 仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { createInitialCallStackState, reduceCallStackEvent, stackSafe } from './kernel';
import { callStackCodeTrace, callStackNarrative } from './trace';
import { renderCallStackView } from './view';
import type { CallStackState } from './model';

export const evmCallStackSimulation: SimPackage<CallStackState> = {
  meta: { code: 'builtin__runtime-evm-call-stack', name: 'EVM 调用栈与 revert 推演', category: 'transaction-runtime', version: '1.0.0', compute: 'frontend', summary: '完整推演 EVM 外部调用、栈帧压入、返回值传播、revert 冒泡和调用深度保护。', learningObjectives: ['理解合约调用栈结构', '掌握返回值和 revert 传播', '观察深度保护如何阻断递归'], scaleLimit: { nodes: 64, maxTick: 120, maxEvents: 220 } },
  initState: createInitialCallStackState,
  reducer: reduceCallStackEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择合约', description: '查看调用栈中的合约状态。', emits: 'select', target: 'element', elementFilter: 'runtime-actor' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '推进调用栈流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '触发 revert', description: '让深层调用失败并向上冒泡。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '处理返回值', description: '上层合约检查返回值并恢复。', emits: 'recover', labelTag: 'perturb' }],
  render: renderCallStackView,
  narrative: callStackNarrative,
  codeTrace: callStackCodeTrace,
  checkpoints: [{ id: 'call-stack-safe', label: '调用栈已安全收敛', evaluate: (state) => stackSafe(state as CallStackState) }],
};
