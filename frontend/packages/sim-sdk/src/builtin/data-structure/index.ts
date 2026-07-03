// 本文件聚合链上数据结构内置仿真包,每个数据结构的状态机实现位于独立目录。

import type { SimPackage } from '../../types';
import { blockchainLinkSimulation } from './blockchain-link/package';
import { merkleTreeStructureSimulation } from './merkle-tree-structure/package';
import { patriciaTrieSimulation } from './patricia-trie/package';
import { stateSnapshotSimulation } from './state-snapshot/package';
import { utxoSetSimulation } from './utxo-set/package';

export const dataStructureSimulations: SimPackage[] = [
  blockchainLinkSimulation as unknown as SimPackage,
  merkleTreeStructureSimulation as unknown as SimPackage,
  patriciaTrieSimulation as unknown as SimPackage,
  utxoSetSimulation as unknown as SimPackage,
  stateSnapshotSimulation as unknown as SimPackage,
];
