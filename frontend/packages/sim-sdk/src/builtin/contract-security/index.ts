// 本文件聚合合约安全内置仿真包,每个漏洞机制的状态机实现位于独立目录。

import type { SimPackage } from '../../types';
import { accessControlSimulation } from './access-control/package';
import { flashLoanSimulation } from './flash-loan/package';
import { integerBoundarySimulation } from './integer-boundary/package';
import { oracleManipulationSimulation } from './oracle-manipulation/package';
import { reentrancySimulation } from './reentrancy/package';

export const contractSecuritySimulations: SimPackage[] = [
  reentrancySimulation as unknown as SimPackage,
  accessControlSimulation as unknown as SimPackage,
  oracleManipulationSimulation as unknown as SimPackage,
  flashLoanSimulation as unknown as SimPackage,
  integerBoundarySimulation as unknown as SimPackage,
];
