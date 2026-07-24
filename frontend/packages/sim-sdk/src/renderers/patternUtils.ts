// patternUtils 提供各渲染器共享的过程进度、焦点状态和选择交互。

import React from 'react';
import { triggerHaptic } from '@chaimir/ui';
import type { FrameFocus, LaneMessage } from '../types';

/**
 * messageTimePosition 将消息发送、到达和过程进度映射为泳道中的连续位置。
 */
export function messageTimePosition(message: LaneMessage, maxTime: number, reducedMotion: boolean): number {
  const duration = Math.max(0, (message.endAt ?? message.at) - message.at);
  const progress = reducedMotion ? staticMessageProgress(message) : clamp01(message.process?.progress ?? 1);
  const logicalTime = duration > 0 ? message.at + duration * progress : message.at;
  return Math.min(92, Math.max(0, (logicalTime / maxTime) * 88));
}

/**
 * processSpanProgress 在减少动态时只保留离散状态,不播放连续过程。
 */
export function processSpanProgress(process: { progress: number }, reducedMotion: boolean): number {
  return reducedMotion ? Math.round(clamp01(process.progress)) : clamp01(process.progress);
}

/**
 * isActiveProcess 只保留尚未完成的过程效果,完成后的对象仍由状态与生命周期表达。
 */
export function isActiveProcess(process: { progress: number }): boolean {
  const progress = clamp01(process.progress);
  return progress > 0 && progress < 1;
}

/**
 * staticMessageProgress 在减少动态时把泳道消息固定到起点或终点,避免滑行动效。
 */
export function staticMessageProgress(message: LaneMessage): number {
  return message.status === 'sent' ? 0 : 1;
}

/**
 * clamp01 约束过程进度,防止仿真包错误数据撑破渲染布局。
 */
export function clamp01(value: number): number {
  return Math.min(1, Math.max(0, Number.isFinite(value) ? value : 0));
}

/**
 * elementVisualClasses 按 TeachingFrame 焦点和元素生命周期生成跨模式通用视觉状态。
 */
export function elementVisualClasses(element: { id: string; meta?: { lifecycle?: { state: string }; emphasis?: string } }, focus?: FrameFocus): string[] {
  const classes: string[] = [];
  if (focus?.primary.includes(element.id)) {
    classes.push('is-focus');
  }
  if (focus?.secondary?.includes(element.id)) {
    classes.push('is-context');
  }
  if (focus?.muted?.includes(element.id)) {
    classes.push('is-muted');
  }
  if (element.meta?.lifecycle?.state) {
    classes.push(`is-${element.meta.lifecycle.state}`);
  }
  if (element.meta?.emphasis) {
    classes.push(`is-${element.meta.emphasis}`);
  }
  return classes;
}

/**
 * 让渲染器内的语义元素可以被键盘和鼠标选中,供 select-element 交互传递目标。
 */
export function selectableElementProps(elementId: string, onSelectElement?: (elementId: string, elementType?: string) => void, elementType?: string) {
  if (!onSelectElement) {
    return {};
  }
  return {
    role: 'button',
    tabIndex: 0,
    'aria-label': `选择${elementType ?? '元素'} ${elementId}`,
    onClick: () => {
      triggerHaptic();
      onSelectElement(elementId, elementType);
    },
    onKeyDown: (event: React.KeyboardEvent) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        triggerHaptic();
        onSelectElement(elementId, elementType);
      }
    },
  };
}
