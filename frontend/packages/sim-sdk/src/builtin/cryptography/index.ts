// 本文件聚合密码学内置仿真包,每个密码学机制的实现位于独立目录。

import type { SimPackage } from '../../types';
import { digitalSignatureSimulation } from './digital-signature/package';
import { hashChainSimulation } from './hash-chain/package';
import { merkleProofSimulation } from './merkle-proof/package';
import { thresholdSignatureSimulation } from './threshold-signature/package';
import { zkProofSimulation } from './zk-proof/package';

export const cryptographySimulations: SimPackage[] = [
  hashChainSimulation as unknown as SimPackage,
  digitalSignatureSimulation as unknown as SimPackage,
  merkleProofSimulation as unknown as SimPackage,
  zkProofSimulation as unknown as SimPackage,
  thresholdSignatureSimulation as unknown as SimPackage,
];
