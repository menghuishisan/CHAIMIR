// 本文件把延迟丢包内核状态映射为封闭可视化模式。

import type { MatrixCell, TeachingFrame } from '../../../types';
import { teachingFrame, chartPattern, lanePattern, matrixPattern, selectedOrFrameFocus } from '../../packageTools';
import { laneMessages, matrixCells, metricSeries } from '../networkView';
import { averageLatency, deliveredCount } from './kernel';
import type { LatencyLossState } from './model';

/**
 * renderLatencyLossView 输出数据包时序、包状态矩阵和传输趋势。
 */
export function renderLatencyLossView(state: LatencyLossState): TeachingFrame {
  const dropped = state.packets.filter((packet) => packet.dropped).length;
    const summary = `拥塞窗口 ${state.congestionWindow},已送达 ${deliveredCount(state)}/${state.packets.length},丢包 ${dropped},平均延迟 ${averageLatency(state)}ms。`;
  const patterns = [
      lanePattern('loss-lane', '数据包发送、ACK 与重传时序', ['发送端', '接收端'], laneMessages(state.messages, labelOf), state.tick),
      matrixPattern('loss-matrix', '包级 ACK / 丢包 / 重试矩阵', state.packets.map((packet) => `包 ${packet.seq}`), ['发送', 'ACK', '丢包', '重试'], packetCells(state)),
      chartPattern('loss-chart', '吞吐覆盖率 / 风险 / 延迟趋势', metricSeries(state.samples), '%'),
    ];
  return teachingFrame({
    summary,
    phase: {
      id: state.phase,
      title: state.explanation.title,
      intent: 'observe',
      what: state.explanation.effect,
      why: state.explanation.reason,
      watch: summary,
    },
    focus: {
      primary: selectedOrFrameFocus(state.selectedElementId, ['loss-lane']),
      secondary: ['loss-matrix', 'loss-chart'],
    },
    layout: {
      primary: 'loss-lane',
      evidence: ['loss-matrix'],
      metrics: ['loss-chart'],
    },
    patterns,
  });
}

/**
 * packetCells 展示包级状态。
 */
function packetCells(state: LatencyLossState): MatrixCell[][] {
  return matrixCells(state.packets.map((packet) => `包 ${packet.seq}`), ['发送', 'ACK', '丢包', '重试'], (row, column) => {
    const packet = state.packets.find((item) => row.endsWith(String(item.seq)));
    if (!packet) return { label: '无', status: 'empty' };
    if (column === '发送') return { label: packet.sent ? '已发' : '等待', status: packet.sent ? 'yes' : 'empty' };
    if (column === 'ACK') return { label: packet.acked ? '已确认' : '等待', status: packet.acked ? 'yes' : 'pending' };
    if (column === '丢包') return { label: packet.dropped ? '丢失' : '无', status: packet.dropped ? 'fault' : 'empty' };
    return { label: String(packet.retry), status: packet.retry > 0 ? 'yes' : 'empty' };
  });
}

/**
 * labelOf 返回泳道名称。
 */
function labelOf(id: string): string {
  if (id === 'sender') return '发送端';
  if (id === 'receiver') return '接收端';
  return id;
}
