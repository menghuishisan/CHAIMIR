// 本文件装配区块链父哈希结构仿真包,内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { blockchainLinkValid, createInitialBlockchainLinkState, reduceBlockchainLinkEvent } from './kernel';
import { blockchainCodeTrace, blockchainNarrative } from './trace';
import { renderBlockchainLinkView } from './view';
import type { BlockchainLinkState } from './model';

export const blockchainLinkSimulation: SimPackage<BlockchainLinkState> = {
  meta: { code: 'builtin__data-blockchain-link', name: '区块链父哈希结构推演', category: 'data-structure', version: '1.0.0', compute: 'frontend', summary: '完整推演区块链创世块、父哈希链接、区块追加、分叉识别与规范链重组。', learningObjectives: ['理解父哈希如何链接区块', '掌握分叉与规范链区别', '观察重组如何改变规范历史'], scaleLimit: { nodes: 96, maxTick: 140, maxEvents: 240 } },
  initState: createInitialBlockchainLinkState,
  reducer: reduceBlockchainLinkEvent,
  interactions: [{ id: 'select', kind: 'select-element', label: '选择区块', description: '选择区块查看链接状态。', emits: 'select', target: 'element', elementFilter: 'block' }, { id: 'advance', kind: 'button', label: '推进阶段', description: '按区块链结构规则推进。', emits: 'advance', labelTag: 'normal' }, { id: 'attack', kind: 'button', label: '制造分叉', description: '在同一父块上追加竞争分支。', emits: 'attack', labelTag: 'attack' }, { id: 'recover', kind: 'button', label: '执行重组', description: '按规范链选择规则完成重组。', emits: 'recover', labelTag: 'perturb' }],
  render: renderBlockchainLinkView,
  narrative: blockchainNarrative,
  codeTrace: blockchainCodeTrace,
  checkpoints: [{ id: 'blockchain-link-valid', label: '父哈希链接有效', evaluate: (state) => blockchainLinkValid(state as BlockchainLinkState) }],
};
