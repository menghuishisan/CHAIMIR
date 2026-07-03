// 本文件实现发送窗口、延迟、丢包检测、重传、退避和窗口恢复内核。

import type { CheckpointResult, ReducerContext, SimEvent, SimInitParams } from '../../../types';
import { deterministicId } from '../../../runtime/deterministic';
import { processNetworkMessage, refreshNetworkMessages, type NetworkMessageView } from '../networkView';
import { latencyLossPhases, type LatencyLossState, type Packet } from './model';
import { traceLinesForLatencyLoss } from './trace';

/**
 * createInitialLatencyLossState 创建四个待发送包和初始拥塞窗口。
 */
export function createInitialLatencyLossState(_params: SimInitParams, _seed: number): LatencyLossState {
  const packets = [1, 2, 3, 4].map<Packet>((seq) => ({ id: `packet-${seq}`, seq, sent: false, delivered: false, acked: false, dropped: false, retry: 0, latencyMs: 0, timeoutAt: 0 }));
  return finalizeLatencyLossState({ tick: 0, phase: latencyLossPhases[0].label, phaseIndex: 0, packets, messages: [], congestionWindow: 2, slowStartThreshold: 3, samples: [{ x: 0, coverage: 0, risk: 10, latency: 0 }], lossInjected: false, lastTransition: 'queue', explanation: explainLatencyLossPhase(0), metrics: {}, checkpointValues: {} });
}

/**
 * reduceLatencyLossEvent 是延迟丢包包唯一事件入口。
 */
export function reduceLatencyLossEvent(state: LatencyLossState, event: SimEvent, _context: ReducerContext): LatencyLossState {
  if (event.type === 'select') return finalizeLatencyLossState({ ...state, selectedElementId: event.target });
  if (event.type === 'attack') return finalizeLatencyLossState(injectLoss(state));
  if (event.type === 'recover') return finalizeLatencyLossState(retransmitLost(state));
  if (event.type === 'advance' || event.type === 'tick') return finalizeLatencyLossState(advanceLatencyLoss(state, event));
  return state;
}

/**
 * advanceLatencyLoss 推进可靠传输阶段。
 */
export function advanceLatencyLoss(state: LatencyLossState, event: SimEvent): LatencyLossState {
  const phaseIndex = Math.min(latencyLossPhases.length - 1, state.phaseIndex + 1);
  const next = { ...state, phaseIndex, tick: event.source === 'tick' ? state.tick + 1 : state.tick, lastTransition: latencyLossPhases[phaseIndex].id };
  if (phaseIndex === 1) return sendWindow(next);
  if (phaseIndex === 2) return detectLoss(next.lossInjected ? next : injectLoss(next));
  if (phaseIndex === 3) return retransmitLost(next);
  if (phaseIndex === 4) return recoverWindow(next);
  return next;
}

/**
 * allDelivered 输出可靠送达检查点。
 */
export function allDelivered(state: LatencyLossState): CheckpointResult {
  const achieved = Boolean(state.checkpointValues.delivered);
  return { achieved, answer: { delivered: deliveredCount(state), total: state.packets.length }, explanation: achieved ? '所有丢失数据包已重传并送达。' : '仍有数据包未送达。' };
}

/**
 * finalizeLatencyLossState 刷新可靠传输指标。
 */
export function finalizeLatencyLossState(state: LatencyLossState): LatencyLossState {
  const delivered = deliveredCount(state);
  const risk = state.packets.some((packet) => packet.dropped) ? 78 : 8;
  const samples = state.samples.concat({ x: state.tick + state.phaseIndex, coverage: Math.round((delivered / state.packets.length) * 100), risk, latency: averageLatency(state) }).slice(-24);
  return {
    ...state,
    phase: latencyLossPhases[state.phaseIndex].label,
    messages: refreshNetworkMessages(state.messages, state.tick, (message) => message.detail ?? '可靠传输消息正在传播。'),
    samples,
    explanation: explainLatencyLossPhase(state.phaseIndex),
    metrics: { result: delivered === state.packets.length ? '全部送达' : '等待重传', risk, delivered },
    checkpointValues: { delivered: delivered === state.packets.length && !state.packets.some((packet) => packet.dropped) },
    _trace: { triggeredLines: traceLinesForLatencyLoss(state.lastTransition), variables: { congestionWindow: state.congestionWindow, delivered, slowStartThreshold: state.slowStartThreshold }, executionPath: `latency-loss/${state.lastTransition}` },
  };
}

/**
 * sendWindow 发送窗口内的数据包并生成 ACK。
 */
function sendWindow(state: LatencyLossState): LatencyLossState {
  let slots = state.congestionWindow;
  const messages: NetworkMessageView[] = [];
  const packets = state.packets.map((packet) => {
    if (!packet.sent && slots > 0) {
      slots -= 1;
      messages.push(message(packet.seq, '数据包', state.tick, false, '数据包进入网络并产生传输延迟。'));
      messages.push(message(packet.seq, 'ACK', state.tick + 1, false, '接收端返回确认。'));
      return { ...packet, sent: true, delivered: true, acked: true, latencyMs: 20 + packet.seq * 5, timeoutAt: state.tick + 2 };
    }
    return packet;
  });
  return { ...state, lastTransition: 'send', packets, messages: state.messages.concat(messages) };
}

/**
 * injectLoss 注入丢包并收缩窗口。
 */
function injectLoss(state: LatencyLossState): LatencyLossState {
  return { ...state, lastTransition: 'loss', lossInjected: true, congestionWindow: 1, slowStartThreshold: Math.max(1, Math.floor(state.congestionWindow / 2)), packets: state.packets.map((packet) => (packet.seq === 2 ? { ...packet, delivered: false, acked: false, dropped: true } : packet)), messages: state.messages.concat(message(2, '丢包', state.tick, true, '包超时未确认,被标记为丢失。')) };
}

/**
 * detectLoss 根据超时状态刷新丢包标记。
 */
function detectLoss(state: LatencyLossState): LatencyLossState {
  return { ...state, lastTransition: 'loss', packets: state.packets.map((packet) => (packet.sent && !packet.acked && state.tick >= packet.timeoutAt ? { ...packet, dropped: true } : packet)) };
}

/**
 * retransmitLost 重传所有丢失包。
 */
function retransmitLost(state: LatencyLossState): LatencyLossState {
  return { ...state, lastTransition: 'retry', packets: state.packets.map((packet) => (packet.dropped ? { ...packet, sent: true, delivered: true, acked: true, dropped: false, retry: packet.retry + 1, latencyMs: packet.latencyMs + 40, timeoutAt: state.tick + 2 } : packet)), messages: state.messages.concat(state.packets.filter((packet) => packet.dropped).map((packet) => message(packet.seq, '重传', state.tick, false, '发送端重传丢失数据包。'))) };
}

/**
 * recoverWindow 在丢包恢复后增长拥塞窗口。
 */
function recoverWindow(state: LatencyLossState): LatencyLossState {
  const allOk = deliveredCount(state) === state.packets.length;
  const nextWindow = allOk ? state.congestionWindow + 1 : Math.max(2, state.congestionWindow + 1);
  return sendRemainingAfterBackoff({ ...state, lastTransition: 'backoff', congestionWindow: Math.min(4, nextWindow) });
}

/**
 * sendRemainingAfterBackoff 在退避恢复后继续发送窗口允许的未发送包。
 */
function sendRemainingAfterBackoff(state: LatencyLossState): LatencyLossState {
  let slots = state.congestionWindow;
  const messages: NetworkMessageView[] = [];
  const packets = state.packets.map((packet) => {
    if (!packet.sent && slots > 0) {
      slots -= 1;
      messages.push(message(packet.seq, '数据包', state.tick, false, '退避恢复后继续发送等待队列。'));
      messages.push(message(packet.seq, 'ACK', state.tick + 1, false, '接收端确认恢复后的数据包。'));
      return { ...packet, sent: true, delivered: true, acked: true, latencyMs: 30 + packet.seq * 6, timeoutAt: state.tick + 2 };
    }
    return packet;
  });
  return { ...state, packets, messages: state.messages.concat(messages) };
}

/**
 * deliveredCount 统计送达包数量。
 */
export function deliveredCount(state: LatencyLossState): number {
  return state.packets.filter((packet) => packet.delivered).length;
}

/**
 * averageLatency 计算已发送包平均延迟。
 */
export function averageLatency(state: LatencyLossState): number {
  const sent = state.packets.filter((packet) => packet.sent);
  return sent.length === 0 ? 0 : Math.round(sent.reduce((sum, packet) => sum + packet.latencyMs, 0) / sent.length);
}

/**
 * message 创建传输消息。
 */
function message(seq: number, label: string, at: number, dropped: boolean, detail: string): NetworkMessageView {
  const from = label === 'ACK' ? 'receiver' : 'sender';
  const to = label === 'ACK' ? 'sender' : 'receiver';
  return processNetworkMessage(at, { id: deterministicId('loss-msg', { seq, label, at, dropped }), from, to, at, label: `${label} #${seq}`, status: dropped ? 'dropped' : 'delivered' }, detail);
}

/**
 * explainLatencyLossPhase 生成阶段说明。
 */
function explainLatencyLossPhase(index: number) {
  const phase = latencyLossPhases[index];
  return { title: phase.label, effect: phase.effect, reason: phase.reason, defaultDurationMs: 1200 };
}
