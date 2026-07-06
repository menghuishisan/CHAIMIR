// 本文件装配 Ethereum PoS 最终性仿真包。

import type { SimPackage } from '../../../types';
import { createInitialEthPosFinalityState, ethPosFinalityCheckpoint, reduceEthPosFinalityEvent } from './kernel';
import type { EthPosFinalityState } from './model';
import { ethPosFinalityCodeTrace, ethPosFinalityNarrative } from './trace';
import { renderEthPosFinalityView } from './view';

export const ethereumPosFinalitySimulation: SimPackage<EthPosFinalityState> = {
  meta: { code: 'builtin__consensus-ethereum-pos-finality', name: 'Ethereum PoS 链头选择与最终性推演', category: 'consensus', version: '1.0.0', compute: 'frontend', summary: '完整推演 slot 出块、LMD-GHOST 链头选择、Casper FFG justified/finalized checkpoint 和延迟投票影响。', learningObjectives: ['区分 head、justified 和 finalized', '理解最新消息驱动的 fork choice', '观察延迟投票为什么不等于最终性回滚'], scaleLimit: { nodes: 96, maxTick: 140, maxEvents: 260 } },
  initState: createInitialEthPosFinalityState,
  reducer: reduceEthPosFinalityEvent,
  interactions: [
    { id: 'select', kind: 'select-element', label: '选择区块或验证者', description: '查看链头、投票和最终性状态。', emits: 'select', target: 'element', elementFilter: 'eth-pos-element' },
    { id: 'advance', kind: 'button', label: '推进 slot', description: '按 Ethereum PoS 规则推进一个阶段。', emits: 'advance', labelTag: 'normal' },
    { id: 'attack', kind: 'button', label: '延迟验证者', description: '让部分 attestation 延迟到达,观察 head 和 finality 区别。', emits: 'attack', labelTag: 'attack' },
    { id: 'recover', kind: 'button', label: '恢复投票', description: '让延迟验证者恢复在线。', emits: 'recover', labelTag: 'perturb' },
  ],
  render: renderEthPosFinalityView,
  narrative: ethPosFinalityNarrative,
  codeTrace: ethPosFinalityCodeTrace,
  checkpoints: [{ id: 'eth-pos-finalized', label: 'PoS head 与 finalized checkpoint 判断正确', evaluate: (state) => ethPosFinalityCheckpoint(state as EthPosFinalityState) }],
};
