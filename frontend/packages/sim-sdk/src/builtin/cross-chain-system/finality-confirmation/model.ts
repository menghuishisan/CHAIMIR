// 本文件定义跨链最终性确认仿真的状态模型和阶段表。

import type { SimState } from '../../../types';

export interface FinalityState extends SimState {
  phaseIndex: number;
  confirmations: number;
  requiredConfirmations: number;
  confirmationStep: number;
  finalityProof: boolean;
  reorgDetected: boolean;
  released: boolean;
  lastTransition: string;
}

export const finalityPhases = [
  { id: 'observe', label: '观察源链区块', detail: '读取事件所在高度', effect: '跨链系统记录消息所在源链区块高度。', reason: '最终性判断必须绑定具体高度。' },
  { id: 'wait', label: '等待确认数', detail: '累计后续区块', effect: '系统等待足够多后续区块降低重组风险。', reason: '概率最终性链需要确认数作为风险控制。' },
  { id: 'prove', label: '提交最终性证明', detail: '证明不可逆', effect: '中继提交源链最终性证明或足够确认信息。', reason: '目标链只能在最终性达标后执行高价值消息。' },
  { id: 'reorg', label: '检测重组风险', detail: '发现源链回滚', effect: '如果源链发生重组,目标链暂停执行。', reason: '未最终确认消息可能在源链消失。' },
  { id: 'release', label: '确认后释放', detail: '执行目标链动作', effect: '最终性满足且无重组时释放资产或执行消息。', reason: '释放动作必须位于最终性闸门之后。' },
];
