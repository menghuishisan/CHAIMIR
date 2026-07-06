// 本文件装配 Tendermint/CometBFT 轮次共识仿真包。

import type { SimPackage } from '../../../types';
import { createInitialTendermintRoundsState, reduceTendermintRoundsEvent, tendermintRoundsCheckpoint } from './kernel';
import type { TendermintRoundsState } from './model';
import { tendermintRoundsCodeTrace, tendermintRoundsNarrative } from './trace';
import { renderTendermintRoundsView } from './view';

export const tendermintRoundsSimulation: SimPackage<TendermintRoundsState> = {
  meta: { code: 'builtin__consensus-tendermint-rounds', name: 'Tendermint 轮次锁定与提交推演', category: 'consensus', version: '1.0.0', compute: 'frontend', summary: '完整推演 proposal、prevote、precommit、commit、timeout/new round 和 lock 约束。', learningObjectives: ['理解 2/3 prevote 与 2/3 precommit 的区别', '观察 lock 如何保护安全性', '理解超时换轮如何保证活性'], scaleLimit: { nodes: 80, maxTick: 140, maxEvents: 260 } },
  initState: createInitialTendermintRoundsState,
  reducer: reduceTendermintRoundsEvent,
  interactions: [
    { id: 'select', kind: 'select-element', label: '选择验证者', description: '查看验证者投票、锁定值和权重。', emits: 'select', target: 'element', elementFilter: 'tendermint-validator' },
    { id: 'advance', kind: 'button', label: '推进 Round', description: '按 Tendermint 阶段推进。', emits: 'advance', labelTag: 'normal' },
    { id: 'attack', kind: 'button', label: '触发超时', description: '模拟验证者离线导致阈值不足。', emits: 'attack', labelTag: 'attack' },
    { id: 'recover', kind: 'button', label: '恢复广播', description: '恢复验证者并携带 valid value 进入下一轮。', emits: 'recover', labelTag: 'perturb' },
  ],
  render: renderTendermintRoundsView,
  narrative: tendermintRoundsNarrative,
  codeTrace: tendermintRoundsCodeTrace,
  checkpoints: [{ id: 'tendermint-commit', label: 'Tendermint 提交条件判断正确', evaluate: (state) => tendermintRoundsCheckpoint(state as TendermintRoundsState) }],
};
