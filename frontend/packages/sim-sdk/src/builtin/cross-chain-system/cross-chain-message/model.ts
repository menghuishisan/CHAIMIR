// 本文件定义跨链消息生命周期仿真的状态模型和阶段表。

import type { SimState } from '../../../types';
import type { CrossActor, CrossMessage } from '../crossChainView';

export interface CrossChainMessageState extends SimState {
  phaseIndex: number;
  messageId: string;
  locked: boolean;
  relayed: boolean;
  verified: boolean;
  executed: boolean;
  actors: CrossActor[];
  messages: CrossMessage[];
  lastTransition: string;
}

export const crossMessagePhases = [
  { id: 'lock', label: '源链锁定资产', detail: '产生事件', effect: '源链合约锁定资产并产生跨链事件。', reason: '跨链消息必须有源链状态作为依据。' },
  { id: 'message', label: '构造跨链消息', detail: '编码目标和载荷', effect: '事件被编码为包含 nonce、目标链和载荷的消息。', reason: '消息字段决定目标链能否唯一验证和执行。' },
  { id: 'relay', label: '中继提交消息', detail: '传递证明', effect: '中继者把消息和源链证明提交到目标链。', reason: '中继只负责传递,不应成为信任根。' },
  { id: 'verify', label: '目标链验证证明', detail: '检查源链事件', effect: '目标链验证消息确实来自源链已确认事件。', reason: '目标链必须独立验证而不是信任中继者。' },
  { id: 'execute', label: '执行并回执', detail: '铸造或释放', effect: '目标链执行消息并记录回执。', reason: '回执让跨链流程具备可追踪的终态。' },
];
