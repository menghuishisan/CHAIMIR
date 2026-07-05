// 本文件定义延迟丢包与可靠重传仿真的代码追踪和教学叙事。

import type { CodeTraceDef, NarrativeStep } from '../../../types';
import { latencyLossPhases } from './model';

export const latencyLossSource = [
  'function reliableSend(packets) {',
  '  sendUpTo(congestionWindow);',
  '  waitForAckOrDeadline();',
  '  if deadlineExceeded: markLost(packet);',
  '  retransmit(lostPackets);',
  '  congestionWindow = backoffOrGrow();',
  '  require(allDelivered());',
  '}',
];

/**
 * traceLinesForLatencyLoss 把可靠传输迁移映射到伪代码行。
 */
export function traceLinesForLatencyLoss(transition: string): number[] {
  const mapping: Record<string, number[]> = {
    queue: [1],
    send: [2],
    loss: [3, 4],
    retry: [5],
    backoff: [6, 7],
  };
  return mapping[transition] ?? [1];
}

export const latencyLossCodeTrace: CodeTraceDef = {
  sourceCode: latencyLossSource.join('\n'),
  language: 'pseudocode',
  lineMapping: [
    { line: 2, triggerCondition: 'lastTransition == send', annotation: '发送端只发送窗口允许数量的数据包。' },
    { line: 4, triggerCondition: 'lastTransition == loss', annotation: '超时未确认的数据包被标记为丢失。', highlightStyle: 'error' },
    { line: 5, triggerCondition: 'lastTransition == retry', annotation: '丢失包会重新发送并等待 ACK。', highlightStyle: 'success' },
    { line: 6, triggerCondition: 'lastTransition == backoff', annotation: '发生丢包后窗口退避,稳定后再增长。' },
    { line: 7, triggerCondition: 'lastTransition == backoff', annotation: '所有包送达才满足可靠传输。', highlightStyle: 'success' },
  ],
  variableWatch: [
    { name: 'congestionWindow', extract: 'state.congestionWindow', format: 'number' },
    { name: 'delivered', extract: 'state.metrics.delivered', format: 'number' },
    { name: 'slowStartThreshold', extract: 'state.slowStartThreshold', format: 'number' },
  ],
};

export const latencyLossNarrative: NarrativeStep[] = latencyLossPhases.map((phase, index) => ({
  id: phase.id,
  title: phase.label,
  trigger: (state) => state.phase === phase.label,
  highlight: [phase.id],
  explain: `${phase.effect} ${phase.reason}`,
  defaultDurationMs: 1200,
  question:
    index === latencyLossPhases.length - 1
      ? {
          prompt: '当前丢失的数据包是否已经可靠重传并完成窗口恢复?',
          options: ['已经完成', '还没有'],
          answer: '已经完成',
          checkpointId: 'latency-loss-delivered',
        }
      : undefined,
}));
