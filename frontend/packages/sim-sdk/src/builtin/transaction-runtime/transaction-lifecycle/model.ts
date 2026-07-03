// 本文件定义交易生命周期仿真的状态模型和阶段表。

import type { SimState } from '../../../types';
import type { RuntimeActor, RuntimeMessage } from '../runtimeView';

export interface TxLifecycleState extends SimState {
  phaseIndex: number;
  txHash: string;
  signed: boolean;
  inMempool: boolean;
  included: boolean;
  executed: boolean;
  receipt: string;
  dropped: boolean;
  actors: RuntimeActor[];
  messages: RuntimeMessage[];
  lastTransition: string;
}

export const txLifecyclePhases = [
  { id: 'build', label: '构造交易', detail: '填写 to/value/data', effect: '用户创建包含目标、金额、数据、gas 和 nonce 的交易。', reason: '交易字段决定后续验签、排序和执行语义。' },
  { id: 'sign', label: '本地签名', detail: '私钥签名', effect: '钱包用私钥签名交易并生成哈希。', reason: '签名证明交易来自账户控制者。' },
  { id: 'mempool', label: '进入交易池', detail: '节点初筛', effect: '节点校验签名、余额和 nonce 后把交易放入 mempool。', reason: '交易池只接收可执行候选交易。' },
  { id: 'include', label: '区块打包', detail: '矿工或验证者选择', effect: '出块者按费用和有效性选择交易写入区块。', reason: '进入区块才开始改变链上状态。' },
  { id: 'execute', label: '执行并生成回执', detail: '状态转移', effect: '运行时执行交易并输出成功或失败回执。', reason: '回执是用户判断交易结果的最终依据。' },
];
