// 本文件定义延迟丢包与可靠重传仿真的领域模型和阶段元数据。

import type { SimState } from '../../../types';
import type { NetworkMessageView } from '../networkView';

export interface Packet {
  id: string;
  seq: number;
  sent: boolean;
  delivered: boolean;
  acked: boolean;
  dropped: boolean;
  retry: number;
  latencyMs: number;
  timeoutAt: number;
}

export interface LatencyLossState extends SimState {
  phaseIndex: number;
  packets: Packet[];
  messages: NetworkMessageView[];
  congestionWindow: number;
  slowStartThreshold: number;
  lossSeq: number;
  samples: Array<{ x: number; coverage: number; risk: number; latency: number }>;
  lossInjected: boolean;
  lastTransition: string;
}

export const latencyLossPhases = [
  { id: 'queue', label: '发送队列排队', detail: '按序号等待发送', effect: '发送端把待发送包放入队列并受窗口限制。', reason: '窗口大小决定同一时间能在网络中的包数量。' },
  { id: 'send', label: '传输数据包', detail: '发送窗口内包', effect: '窗口内数据包进入网络并产生传输延迟。', reason: '延迟不是错误,但会影响确认到达时间。' },
  { id: 'loss', label: '检测丢包', detail: '超时未确认', effect: '未收到确认的数据包被判定为丢失。', reason: '丢包需要靠超时或重复确认显式识别。' },
  { id: 'retry', label: '重传丢失包', detail: '重新发送', effect: '发送端重传丢失包并记录重试次数。', reason: '可靠传输靠重传弥补不可靠网络。' },
  { id: 'backoff', label: '退避恢复窗口', detail: '降低并恢复窗口', effect: '发生丢包后窗口收缩,恢复稳定后再增大。', reason: '退避避免在拥塞网络中继续制造更多丢包。' },
] as const;
