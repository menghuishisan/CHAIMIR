// 本文件聚合交易与执行运行时内置仿真包,每个运行时机制的状态机实现位于独立目录。

import type { SimPackage } from '../../types';
import { blockValidationSimulation } from './block-validation/package';
import { eip1559FeeMarketSimulation } from './eip1559-fee-market/package';
import { evmCallStackSimulation } from './evm-call-stack/package';
import { gasMeteringSimulation } from './gas-metering/package';
import { mempoolReplacementSimulation } from './mempool-replacement/package';
import { nonceOrderingSimulation } from './nonce-ordering/package';
import { transactionLifecycleSimulation } from './transaction-lifecycle/package';

export const transactionRuntimeSimulations: SimPackage[] = [
  transactionLifecycleSimulation as unknown as SimPackage,
  nonceOrderingSimulation as unknown as SimPackage,
  mempoolReplacementSimulation as unknown as SimPackage,
  gasMeteringSimulation as unknown as SimPackage,
  eip1559FeeMarketSimulation as unknown as SimPackage,
  evmCallStackSimulation as unknown as SimPackage,
  blockValidationSimulation as unknown as SimPackage,
];
