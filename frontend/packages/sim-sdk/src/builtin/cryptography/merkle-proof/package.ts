// 本文件装配 Merkle 证明仿真包,具体内核、视图和追踪由同目录文件维护。

import type { SimPackage } from '../../../types';
import { commonAlgorithmInteractions } from '../../packageTools';
import { createInitialMerkleProofState, merkleProofValid, reduceMerkleProofEvent } from './kernel';
import { merkleProofCodeTrace, merkleProofNarrative } from './trace';
import { renderMerkleProofView } from './view';
import type { MerkleProofState } from './model';

export const merkleProofSimulation: SimPackage<MerkleProofState> = {
  meta: {
    code: 'builtin__crypto-merkle-proof',
    name: 'Merkle 证明路径推演',
    category: 'cryptography',
    version: '1.0.0',
    compute: 'frontend',
    summary: '完整推演 Merkle 叶子哈希、兄弟路径选择、根摘要重建、证明校验与篡改定位。',
    learningObjectives: ['理解 Merkle 根如何承诺多笔数据', '掌握证明路径为什么只需兄弟哈希', '观察篡改如何导致根不匹配'],
    scaleLimit: { nodes: 64, maxTick: 120, maxEvents: 200 },
  },
  initState: createInitialMerkleProofState,
  reducer: reduceMerkleProofEvent,
  interactions: commonAlgorithmInteractions('merkle-leaf'),
  render: renderMerkleProofView,
  narrative: merkleProofNarrative,
  codeTrace: merkleProofCodeTrace,
  checkpoints: [{ id: 'merkle-proof-valid', label: 'Merkle 证明通过', evaluate: (state) => merkleProofValid(state as MerkleProofState) }],
};
