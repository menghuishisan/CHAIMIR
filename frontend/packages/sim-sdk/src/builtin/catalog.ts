// 本文件只负责聚合已经按算法或机制单独建模的内置仿真包,不再用通用模板批量伪造场景。

import type { SimPackage } from '../types';
import { consensusSimulations } from './consensus';
import { contractSecuritySimulations } from './contract-security';
import { crossChainSystemSimulations } from './cross-chain-system';
import { cryptographySimulations } from './cryptography';
import { dataStructureSimulations } from './data-structure';
import { networkSimulations } from './network';
import { transactionRuntimeSimulations } from './transaction-runtime';

/**
 * builtinSimulations 只聚合已经完成独立算法建模的内置仿真包。
 */
export const builtinSimulations: SimPackage[] = [
  ...consensusSimulations,
  ...cryptographySimulations,
  ...networkSimulations,
  ...dataStructureSimulations,
  ...contractSecuritySimulations,
  ...transactionRuntimeSimulations,
  ...crossChainSystemSimulations,
];
