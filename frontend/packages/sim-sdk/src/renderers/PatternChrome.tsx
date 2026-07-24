// PatternChrome 渲染各可视化模式共享的标题、摘要和图例。

import React from 'react';
import { BarChart3, GitBranch, Layers3, Network, Rows3, Table2, Workflow } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import type { PatternBinding } from '../types';

/**
 * 渲染统一模式标题和当前数据摘要。
 */
export function PatternHeader({ mode, title, meta }: { mode: PatternBinding['mode']; title: string; meta: string }): React.ReactElement {
  const Icon = patternModeIcon(mode);
  return (
    <header className="sim-pattern__header">
      <span>
        <Icon size={15} aria-hidden="true" />
        {title}
      </span>
      <small>{meta}</small>
    </header>
  );
}

/**
 * PatternInsight 用紧凑键值对补充每种模式的过程指标,避免把关键信息塞进图形内部。
 */
export function PatternInsight({ items }: { items: Array<[string, string | number]> }): React.ReactElement {
  return (
    <dl className="sim-pattern__insight">
      {items.map(([label, value]) => (
        <div key={label}>
          <dt>{label}</dt>
          <dd>{value}</dd>
        </div>
      ))}
    </dl>
  );
}

/**
 * patternModeIcon 为封闭模式提供稳定图标,让不同仿真视图的语义差异先于颜色被识别。
 */
function patternModeIcon(mode: PatternBinding['mode']): LucideIcon {
  const icons: Record<PatternBinding['mode'], LucideIcon> = {
    graph: Network,
    chain: GitBranch,
    tree: Layers3,
    matrix: Table2,
    pipeline: Workflow,
    lane: Rows3,
    chart: BarChart3,
  };
  return icons[mode];
}

/**
 * 渲染色彩之外的图例,满足可视化不能只靠颜色表达状态的要求。
 */
export function PatternLegend({ items }: { items: Array<[string, 'accent' | 'danger' | 'muted' | 'success']> }): React.ReactElement {
  return (
    <div className="sim-pattern__legend" aria-label="图例">
      {items.map(([label, tone]) => (
        <span className={`is-${tone}`} key={label}>
          <i aria-hidden="true" />
          {label}
        </span>
      ))}
    </div>
  );
}
