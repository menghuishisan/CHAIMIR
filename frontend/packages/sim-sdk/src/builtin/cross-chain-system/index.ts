// 本文件聚合跨链与系统机制内置仿真包,每个跨链机制的状态机实现位于独立目录。

import type { SimPackage } from '../../types';
import { bridgeValidationSimulation } from './bridge-validation/package';
import { crossChainMessageSimulation } from './cross-chain-message/package';
import { finalityConfirmationSimulation } from './finality-confirmation/package';
import { multisigCommitteeSimulation } from './multisig-committee/package';
import { replayProtectionSimulation } from './replay-protection/package';

export const crossChainSystemSimulations: SimPackage[] = [
  crossChainMessageSimulation as unknown as SimPackage,
  bridgeValidationSimulation as unknown as SimPackage,
  multisigCommitteeSimulation as unknown as SimPackage,
  finalityConfirmationSimulation as unknown as SimPackage,
  replayProtectionSimulation as unknown as SimPackage,
];
