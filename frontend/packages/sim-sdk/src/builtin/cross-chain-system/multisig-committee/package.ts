// 本文件装配跨链多签委员会仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { committeeAuthorized, createInitialCommitteeState, reduceCommitteeEvent } from './kernel';
import { committeeCodeTrace, committeeNarrative } from './trace';
import { renderCommitteeView } from './view';
import type { CommitteeState } from './model';

export const multisigCommitteeSimulation: SimPackage<CommitteeState> = {
  meta: { code: 'builtin__cross-multisig-committee', name: '跨链多签委员会推演', category: 'cross-chain-system', version: '1.0.0', compute: 'frontend', summary: '完整推演跨链多签委员会轮换、成员签名、门限聚合、恶意签名剔除和执行授权。', learningObjectives: ['理解多签委员会的门限授权', '掌握活跃成员校验', '观察恶意签名为何不能计入门限'], scaleLimit: { nodes: 64, maxTick: 120, maxEvents: 220 } },
  initState: createInitialCommitteeState,
  reducer: reduceCommitteeEvent,
  interactions: [{ id: 'advance', kind: 'button', label: '推进阶段', description: '推进多签委员会流程。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '注入恶意签名', description: '让非可信成员提交签名。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '过滤签名', description: '剔除非活跃或恶意签名。', emits: 'recover', labelTag: 'perturb' }],
  render: renderCommitteeView,
  narrative: committeeNarrative,
  codeTrace: committeeCodeTrace,
  checkpoints: [{ id: 'committee-authorized', label: '多签门限授权通过', evaluate: (state) => committeeAuthorized(state as CommitteeState) }],
};
