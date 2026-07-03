// 本文件定义跨链桥证明验证仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface BridgeState extends SimState {
  phaseIndex: number;
  proofHash: string;
  lightClientSynced: boolean;
  minted: boolean;
  redeemed: boolean;
  invalidProof: boolean;
  lastTransition: string;
}

export const bridgePhases = [
  { id: 'lock', label: '源链锁仓', detail: '生成锁定证明', effect: '用户在源链桥合约锁定资产。', reason: '目标链铸造必须有源链锁仓证明。' },
  { id: 'sync', label: '同步轻客户端', detail: '更新源链头', effect: '目标链桥同步源链区块头和最终性信息。', reason: '轻客户端是目标链验证源链证明的信任根。' },
  { id: 'verify', label: '验证锁定证明', detail: '校验包含关系', effect: '桥合约验证锁定事件包含在已确认源链区块中。', reason: '只验证中继提交的证明,不信任中继身份。' },
  { id: 'mint', label: '目标链铸造', detail: '发行映射资产', effect: '验证通过后目标链铸造等额映射资产。', reason: '锁仓与铸造必须一一对应。' },
  { id: 'redeem', label: '赎回销毁', detail: '反向释放', effect: '用户销毁目标链资产并在源链释放锁仓。', reason: '赎回闭环保证跨链资产供应一致。' },
];
