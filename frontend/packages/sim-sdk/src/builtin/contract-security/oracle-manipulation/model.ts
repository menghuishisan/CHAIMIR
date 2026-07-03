// 本文件定义预言机操纵仿真的状态模型和阶段表。

import type { SimState } from '../../../types';
import type { SecurityActor, SecurityCall } from '../securityView';

export interface OracleState extends SimState {
  phaseIndex: number;
  spotPrice: number;
  twapPrice: number;
  referencePrice: number;
  manipulationActive: boolean;
  actors: SecurityActor[];
  calls: SecurityCall[];
  lastTransition: string;
}

export const oraclePhases = [
  { id: 'read', label: '读取现货价格', detail: '从池子取价', effect: '借贷合约直接读取 AMM 当前价格。', reason: '现货价格容易被同区块大额交易短暂推偏。' },
  { id: 'swap', label: '低流动性大额交易', detail: '推动池内比例', effect: '攻击者通过大额兑换推高或压低池内价格。', reason: '流动性越低,相同资金造成的价格偏移越大。' },
  { id: 'borrow', label: '按偏移价格借款', detail: '高估抵押物', effect: '借贷合约按被操纵价格计算可借额度。', reason: '错误价格会直接放大坏账风险。' },
  { id: 'twap', label: 'TWAP 校验', detail: '时间加权均价', effect: '合约把现货价与时间加权均价比较。', reason: 'TWAP 能削弱瞬时操纵的影响。' },
  { id: 'aggregate', label: '多源聚合修复', detail: '比较多个来源', effect: '价格取多源中位数并设置偏离阈值。', reason: '多源聚合避免单一池子成为安全边界。' },
];
