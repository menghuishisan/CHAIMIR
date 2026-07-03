// 本文件定义 EVM 调用栈仿真的状态模型和阶段表。

import type { SimState } from '../../../types';
import type { RuntimeActor, RuntimeMessage } from '../runtimeView';

export interface Frame {
  id: string;
  contract: string;
  depth: number;
  returned: boolean;
  reverted: boolean;
}

export interface CallStackState extends SimState {
  phaseIndex: number;
  frames: Frame[];
  maxDepth: number;
  actors: RuntimeActor[];
  messages: RuntimeMessage[];
  lastTransition: string;
}

export const callStackPhases = [
  { id: 'external', label: '外部账户发起调用', detail: 'EOA 调合约 A', effect: '外部账户向入口合约发送交易调用。', reason: '调用栈从交易入口开始建立。' },
  { id: 'push', label: '压入调用栈帧', detail: 'A 调 B', effect: '每次合约调用都会压入新的执行栈帧。', reason: '栈帧保存调用上下文、返回位置和局部状态。' },
  { id: 'return', label: '返回值传播', detail: 'B 返回 A', effect: '被调用合约返回数据给上一层调用者。', reason: '上层合约必须检查返回值而不是假设成功。' },
  { id: 'revert', label: 'revert 冒泡', detail: '失败向上传播', effect: '底层失败会向上层调用冒泡并回滚相关状态。', reason: '未处理 revert 会让整笔交易失败。' },
  { id: 'depth', label: '深度保护', detail: '限制递归调用', effect: '运行时限制调用深度并拒绝过深递归。', reason: '深度限制防止无限递归消耗资源。' },
];
