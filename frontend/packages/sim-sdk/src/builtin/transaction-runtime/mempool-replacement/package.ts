// 本文件装配 Mempool 替换与 nonce 队列仿真包。

import type { SimPackage } from '../../../types';
import { createInitialMempoolReplacementState, mempoolReplacementCheckpoint, reduceMempoolReplacementEvent } from './kernel';
import type { MempoolReplacementState } from './model';
import { mempoolReplacementCodeTrace, mempoolReplacementNarrative } from './trace';
import { renderMempoolReplacementView } from './view';

export const mempoolReplacementSimulation: SimPackage<MempoolReplacementState> = {
  meta: { code: 'builtin__runtime-mempool-replacement', name: 'Mempool 替换交易与 Nonce 队列推演', category: 'transaction-runtime', version: '1.0.0', compute: 'frontend', summary: '完整推演 pending/queued 划分、同 nonce 替换阈值、节点视图传播和区块打包释放队列。', learningObjectives: ['理解 nonce 缺口为什么阻塞后续交易', '掌握替换交易必须足额加价', '区分 mempool 本地视图和链上顺序'], scaleLimit: { nodes: 72, maxTick: 120, maxEvents: 240 } },
  initState: createInitialMempoolReplacementState,
  reducer: reduceMempoolReplacementEvent,
  interactions: [
    { id: 'select', kind: 'select-element', label: '选择交易', description: '查看交易 nonce、费用和当前池状态。', emits: 'select', target: 'element', elementFilter: 'pool-tx' },
    { id: 'advance', kind: 'button', label: '推进交易池', description: '按交易池规则推进一个阶段。', emits: 'advance', labelTag: 'normal' },
    { id: 'attack', kind: 'button', label: '低价替换', description: '提交加价不足的同 nonce 替换交易。', emits: 'attack', labelTag: 'attack' },
    { id: 'recover', kind: 'button', label: '足额替换', description: '提交满足阈值的高价替换交易。', emits: 'recover', labelTag: 'perturb' },
  ],
  render: renderMempoolReplacementView,
  narrative: mempoolReplacementNarrative,
  codeTrace: mempoolReplacementCodeTrace,
  checkpoints: [{ id: 'mempool-replacement-valid', label: '替换交易与队列释放规则正确', evaluate: (state) => mempoolReplacementCheckpoint(state as MempoolReplacementState) }],
};
