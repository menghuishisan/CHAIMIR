// 本文件聚合跨链与系统机制内置仿真包,每个跨链机制的状态机实现位于独立目录。

import type { SimPackage } from '../../types';
import { bridgeValidationSimulation } from './bridge-validation/package';
import { crossChainMessageSimulation } from './cross-chain-message/package';
import { finalityConfirmationSimulation } from './finality-confirmation/package';
import { multisigCommitteeSimulation } from './multisig-committee/package';
import { optimisticRollupFraudProofSimulation } from './optimistic-rollup-fraud-proof/package';
import { replayProtectionSimulation } from './replay-protection/package';
import { zkRollupProofVerificationSimulation } from './zk-rollup-proof-verification/package';

export const crossChainSystemSimulations: SimPackage[] = [
  crossChainMessageSimulation,
  bridgeValidationSimulation,
  optimisticRollupFraudProofSimulation,
  zkRollupProofVerificationSimulation,
  multisigCommitteeSimulation,
  finalityConfirmationSimulation,
  replayProtectionSimulation,
];
