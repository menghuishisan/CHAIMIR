// 本文件将仿真包输出的封闭可视化模式语义数据渲染为统一界面,避免各仿真包自带 DOM 或自定义渲染器。

import React from 'react';
import { clsx } from 'clsx';
import type {
  ChainPattern,
  ChartSeries,
  ChartPattern,
  GraphEdge,
  GraphNode,
  GraphPattern,
  LaneMessage,
  LanePattern,
  MatrixPattern,
  PatternBinding,
  PipelinePattern,
  TreeNode,
  TreePattern,
} from '../types';
import './PatternRenderer.css';

export interface PatternRendererProps {
  pattern: PatternBinding;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}

/**
 * 按模式类型分发到平台维护的统一渲染器。
 */
export function PatternRenderer({ pattern, selectedElementId, onSelectElement }: PatternRendererProps): React.ReactElement {
  switch (pattern.mode) {
    case 'graph':
      return <GraphRenderer pattern={pattern} selectedElementId={selectedElementId} onSelectElement={onSelectElement} />;
    case 'chain':
      return <ChainRenderer pattern={pattern} selectedElementId={selectedElementId} onSelectElement={onSelectElement} />;
    case 'tree':
      return <TreeRenderer pattern={pattern} selectedElementId={selectedElementId} onSelectElement={onSelectElement} />;
    case 'matrix':
      return <MatrixRenderer pattern={pattern} selectedElementId={selectedElementId} onSelectElement={onSelectElement} />;
    case 'pipeline':
      return <PipelineRenderer pattern={pattern} selectedElementId={selectedElementId} onSelectElement={onSelectElement} />;
    case 'lane':
      return <LaneRenderer pattern={pattern} selectedElementId={selectedElementId} onSelectElement={onSelectElement} />;
    case 'chart':
      return <ChartRenderer pattern={pattern} selectedElementId={selectedElementId} onSelectElement={onSelectElement} />;
  }
}

/**
 * 渲染节点、连线和消息流,用于共识、网络传播、状态机等图网络场景。
 */
function GraphRenderer({
  pattern,
  selectedElementId,
  onSelectElement,
}: {
  pattern: GraphPattern;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const nodes = layoutGraphNodes(pattern.data.nodes, pattern.data.layout);
  const nodeById = new Map(nodes.map((node) => [node.id, node]));
  const activeEdges = pattern.data.edges.filter((edge) => edge.status === 'active' || edge.status === 'success').length;

  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <PatternHeader title={pattern.title} meta={`${pattern.data.nodes.length} 个节点 / ${activeEdges} 条活跃消息`} />
      <svg className="sim-graph" viewBox="0 0 100 100" role="img" aria-label={`${pattern.title}图网络`}>
        {pattern.data.edges.map((edge) => {
          const from = nodeById.get(edge.from);
          const to = nodeById.get(edge.to);
          if (!from || !to) return null;
          const line = shortenLine(from, to, 6.5);
          const progress = processProgress(edge);
          const pulse = pointOnLine(line, progress);
          return (
            <g className="sim-graph__edge-group" key={edge.id} {...selectableElementProps(edge.id, onSelectElement, 'edge')}>
              <line
                className={clsx('sim-graph__edge', `is-${edge.status}`)}
                x1={line.x1}
                y1={line.y1}
                x2={line.x2}
                y2={line.y2}
              />
              <circle className={clsx('sim-graph__pulse', `is-${edge.status}`)} cx={pulse.x} cy={pulse.y} r="1.3">
                <title>{edge.process?.label ?? edge.detail ?? edge.label}</title>
              </circle>
              <text className="sim-graph__edge-label" x={(from.x + to.x) / 2} y={(from.y + to.y) / 2}>
                {edge.label}
              </text>
            </g>
          );
        })}
        {nodes.map((node) => (
          <g
            key={node.id}
            className={clsx('sim-graph__node', `is-${node.status}`, selectedElementId === node.id && 'is-selected')}
            {...selectableElementProps(node.id, onSelectElement, node.role)}
          >
            <circle cx={node.x} cy={node.y} r="5.8" />
            <text x={node.x} y={node.y + 0.8}>
              {node.label}
            </text>
            {node.value && (
              <text className="sim-graph__node-value" x={node.x} y={node.y + 8.4}>
                {node.value}
              </text>
            )}
          </g>
        ))}
      </svg>
      <PatternLegend items={[['活跃/成功', 'accent'], ['待处理', 'muted'], ['失败/丢弃', 'danger']]} />
    </section>
  );
}

/**
 * 渲染区块主链与分叉序列,用于出块、最长链、双花和自私挖矿场景。
 */
function ChainRenderer({
  pattern,
  selectedElementId,
  onSelectElement,
}: {
  pattern: ChainPattern;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const canonical = pattern.data.blocks;
  const forkBlocks = pattern.data.forks.flat();
  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <PatternHeader title={pattern.title} meta={`${canonical.length} 个主链块 / ${forkBlocks.length} 个分叉块`} />
      <div className="sim-chain-board" role="list" aria-label="主链与分叉">
        <div className="sim-chain-row">
          <span className="sim-chain-row__label">主链</span>
          <div className="sim-chain">
            {canonical.map((block, index) => (
              <React.Fragment key={block.id}>
                {index > 0 && <span className="sim-chain__link" aria-hidden="true" />}
                <ChainBlockCard block={block} selectedElementId={selectedElementId} onSelectElement={onSelectElement} canonicalTip={pattern.data.canonicalTip} />
              </React.Fragment>
            ))}
          </div>
        </div>
        {pattern.data.forks.map((fork, forkIndex) => (
          <div className="sim-chain-row sim-chain-row--fork" key={`fork-${forkIndex}`}>
            <span className="sim-chain-row__label">分叉 {forkIndex + 1}</span>
            <div className="sim-chain">
              {fork.map((block, index) => (
                <React.Fragment key={`${forkIndex}-${block.id}`}>
                  {index > 0 && <span className="sim-chain__link is-fork" aria-hidden="true" />}
                  <ChainBlockCard block={block} selectedElementId={selectedElementId} onSelectElement={onSelectElement} canonicalTip={pattern.data.canonicalTip} />
                </React.Fragment>
              ))}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

/**
 * 渲染树结构及高亮路径,用于 Merkle 树、状态树和证明路径场景。
 */
function TreeRenderer({
  pattern,
  selectedElementId,
  onSelectElement,
}: {
  pattern: TreePattern;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <PatternHeader title={pattern.title} meta={`证明路径 ${pattern.data.highlightedPath.length} 层`} />
      <div className="sim-tree">{renderTreeNode(pattern.data.root, pattern.data.highlightedPath, selectedElementId, onSelectElement, true)}</div>
    </section>
  );
}

/**
 * 递归渲染树节点,保持父子结构与证明路径关系可见。
 */
function renderTreeNode(
  node: TreeNode,
  path: string[],
  selectedElementId?: string,
  onSelectElement?: (elementId: string, elementType?: string) => void,
  isRoot = false
): React.ReactElement {
  return (
    <div className={clsx('sim-tree__node', isRoot && 'is-root', path.includes(node.id) && 'is-highlighted', selectedElementId === node.id && 'is-selected')}>
      <div className="sim-tree__box" {...selectableElementProps(node.id, onSelectElement, 'tree-node')}>
        <span>{node.label}</span>
        <code>{node.hash.slice(0, 10)}</code>
      </div>
      {node.children && node.children.length > 0 && (
        <div className="sim-tree__children">
          {node.children.map((child) => renderTreeNode(child, path, selectedElementId, onSelectElement))}
        </div>
      )}
    </div>
  );
}

/**
 * 渲染投票、校验或状态矩阵,用文字与状态类共同表达结果。
 */
function MatrixRenderer({
  pattern,
  selectedElementId,
  onSelectElement,
}: {
  pattern: MatrixPattern;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const statusCounts = pattern.data.cells.flat().reduce<Record<string, number>>((counts, cell) => {
    counts[cell.status] = (counts[cell.status] ?? 0) + 1;
    return counts;
  }, {});
  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <PatternHeader title={pattern.title} meta={`通过 ${statusCounts.yes ?? 0} / 异常 ${statusCounts.fault ?? 0}`} />
      <div className="sim-matrix-wrap">
        <table className="sim-matrix">
          <thead>
            <tr>
              <th scope="col">对象</th>
              {pattern.data.columns.map((column) => (
                <th scope="col" key={column}>
                  {column}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {pattern.data.rows.map((row, rowIndex) => (
              <tr key={row}>
                <th scope="row">{row}</th>
                {pattern.data.cells[rowIndex].map((cell, columnIndex) => {
                  const cellId = `${pattern.id}:${row}:${pattern.data.columns[columnIndex]}`;
                  return (
                    <td
                      className={clsx('sim-matrix__cell', `is-${cell.status}`, selectedElementId === cellId && 'is-selected')}
                      key={`${row}-${columnIndex}`}
                      {...selectableElementProps(cellId, onSelectElement, 'cell')}
                    >
                      <span>{cell.label}</span>
                      <small>{matrixStatusLabel(cell.status)}</small>
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <PatternLegend items={[['通过', 'success'], ['处理中', 'accent'], ['等待', 'muted'], ['异常', 'danger']]} />
    </section>
  );
}

/**
 * 渲染分阶段数据流,用于哈希、签名、交易生命周期等流水线场景。
 */
function PipelineRenderer({
  pattern,
  selectedElementId,
  onSelectElement,
}: {
  pattern: PipelinePattern;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const complete = pattern.data.steps.filter((step) => step.status === 'complete').length;
  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <PatternHeader title={pattern.title} meta={`${complete}/${pattern.data.steps.length} 步完成`} />
      <ol className="sim-pipeline">
        {pattern.data.steps.map((step, index) => (
          <li
            className={clsx('sim-pipeline__step', `is-${step.status}`, selectedElementId === step.id && 'is-selected')}
            key={step.id}
            {...selectableElementProps(step.id, onSelectElement, 'step')}
          >
            <i>{index + 1}</i>
            <span>{step.label}</span>
            <small>{step.detail}</small>
            {step.process && (
              <span className="sim-pipeline__progress" aria-label={`${step.process.label}${Math.round(step.process.progress * 100)}%`}>
                <span style={{ inlineSize: `${Math.round(clamp01(step.process.progress) * 100)}%` }} />
              </span>
            )}
          </li>
        ))}
      </ol>
    </section>
  );
}

/**
 * 渲染多参与方时序消息,用于共识投票、跨链消息和网络传播场景。
 */
function LaneRenderer({
  pattern,
  selectedElementId,
  onSelectElement,
}: {
  pattern: LanePattern;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const maxTime = Math.max(1, pattern.data.currentTime, ...pattern.data.messages.map((message) => message.endAt ?? message.at));
  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <PatternHeader title={pattern.title} meta={`当前时间 ${pattern.data.currentTime} / 消息 ${pattern.data.messages.length}`} />
      <div className="sim-lane">
        {pattern.data.actors.map((actor) => (
          <div className="sim-lane__row" key={actor}>
            <span className="sim-lane__actor">{actor}</span>
            <div className="sim-lane__messages">
              {pattern.data.messages
                .filter((message) => message.from === actor || message.to === actor)
                .map((message) => {
                  const position = messageTimePosition(message, maxTime);
                  return (
                    <span
                      className={clsx('sim-lane__message', `is-${message.status}`, selectedElementId === message.id && 'is-selected')}
                      key={`${actor}-${message.id}`}
                      style={{ insetInlineStart: `${position}%` }}
                      title={message.process?.label ?? message.detail}
                      {...selectableElementProps(message.id, onSelectElement, 'message')}
                    >
                      <b>{message.from === actor ? '出' : '入'}</b>
                      {message.label}
                      {message.process && <i style={{ inlineSize: `${Math.round(clamp01(message.process.progress) * 100)}%` }} aria-hidden="true" />}
                    </span>
                  );
                })}
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

/**
 * 渲染轻量数值趋势,用于算力、延迟、难度、余额等随步进变化的数据。
 */
function ChartRenderer({
  pattern,
  selectedElementId,
  onSelectElement,
}: {
  pattern: ChartPattern;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const maxY = Math.max(1, ...pattern.data.series.flatMap((series) => series.points.map((point) => point.y)));
  const minX = Math.min(0, ...pattern.data.series.flatMap((series) => series.points.map((point) => point.x)));
  const maxX = Math.max(1, ...pattern.data.series.flatMap((series) => series.points.map((point) => point.x)));
  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <PatternHeader title={pattern.title} meta={`最大值 ${maxY}${pattern.data.unit}`} />
      <div className="sim-chart">
        <svg className="sim-chart__plot" viewBox="0 0 100 60" role="img" aria-label={`${pattern.title}趋势图`}>
          <line className="sim-chart__axis" x1="8" y1="52" x2="96" y2="52" />
          <line className="sim-chart__axis" x1="8" y1="4" x2="8" y2="52" />
          {[25, 50, 75, 100].map((tick) => (
            <g key={tick}>
              <line className="sim-chart__grid" x1="8" x2="96" y1={52 - tick * 0.48} y2={52 - tick * 0.48} />
              <text className="sim-chart__tick" x="2" y={54 - tick * 0.48}>{tick}</text>
            </g>
          ))}
          {pattern.data.series.map((series, index) => (
            <g className={`sim-chart__series-line is-${index % 4}`} key={series.label}>
              <polyline points={chartPolyline(series, minX, maxX, maxY)} />
              {series.points.slice(-16).map((point) => {
                const pointId = `${pattern.id}:${series.label}:${point.x}`;
                const plotted = chartPoint(point, minX, maxX, maxY);
                return (
                  <circle
                    className={clsx(selectedElementId === pointId && 'is-selected')}
                    cx={plotted.x}
                    cy={plotted.y}
                    key={`${series.label}-${point.x}`}
                    r="1.7"
                    {...selectableElementProps(pointId, onSelectElement, 'point')}
                  />
                );
              })}
            </g>
          ))}
        </svg>
        <div className="sim-chart__legend">
          {pattern.data.series.map((series, index) => (
            <span className={`is-${index % 4}`} key={series.label}>
              {series.label}
            </span>
          ))}
        </div>
      </div>
    </section>
  );
}

/**
 * 渲染统一模式标题和当前数据摘要。
 */
function PatternHeader({ title, meta }: { title: string; meta: string }): React.ReactElement {
  return (
    <header className="sim-pattern__header">
      <span>{title}</span>
      <small>{meta}</small>
    </header>
  );
}

/**
 * 渲染色彩之外的图例,满足可视化不能只靠颜色表达状态的要求。
 */
function PatternLegend({ items }: { items: Array<[string, 'accent' | 'danger' | 'muted' | 'success']> }): React.ReactElement {
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

interface PositionedGraphNode extends GraphNode {
  x: number;
  y: number;
}

/**
 * 根据 graph 模式声明的布局类型计算逻辑坐标。
 */
function layoutGraphNodes(nodes: GraphNode[], layout: GraphPattern['data']['layout']): PositionedGraphNode[] {
  if (layout === 'grid') {
    const columns = Math.ceil(Math.sqrt(nodes.length));
    return nodes.map((node, index) => ({
      ...node,
      x: 18 + (index % columns) * (64 / Math.max(1, columns - 1 || 1)),
      y: 18 + Math.floor(index / columns) * (64 / Math.max(1, Math.ceil(nodes.length / columns) - 1 || 1)),
    }));
  }
  if (layout === 'layered') {
    return nodes.map((node, index) => ({
      ...node,
      x: 15 + (index / Math.max(1, nodes.length - 1)) * 70,
      y: index === 0 ? 20 : index === nodes.length - 1 ? 80 : 35 + (index % 2) * 30,
    }));
  }
  return nodes.map((node, index) => {
    const angle = (Math.PI * 2 * index) / Math.max(nodes.length, 1) - Math.PI / 2;
    return {
      ...node,
      x: 50 + Math.cos(angle) * 38,
      y: 50 + Math.sin(angle) * 38,
    };
  });
}

/**
 * 缩短边线,避免连线压到节点圆心和文字。
 */
function shortenLine(from: PositionedGraphNode, to: PositionedGraphNode, offset: number) {
  const dx = to.x - from.x;
  const dy = to.y - from.y;
  const length = Math.max(1, Math.hypot(dx, dy));
  const ox = (dx / length) * offset;
  const oy = (dy / length) * offset;
  return { x1: from.x + ox, y1: from.y + oy, x2: to.x - ox, y2: to.y - oy };
}

/**
 * processProgress 读取协议消息的过程进度,没有过程数据时保持旧的静态位置。
 */
function processProgress(edge: GraphEdge): number {
  if (edge.process) return clamp01(edge.process.progress);
  if (edge.status === 'pending') return 0.15;
  if (edge.status === 'active') return 0.5;
  return 0.85;
}

/**
 * pointOnLine 将 0 到 1 的过程进度映射到 SVG 连线坐标。
 */
function pointOnLine(line: ReturnType<typeof shortenLine>, progress: number): { x: number; y: number } {
  return { x: line.x1 + (line.x2 - line.x1) * progress, y: line.y1 + (line.y2 - line.y1) * progress };
}

/**
 * messageTimePosition 将消息发送、到达和过程进度映射为泳道中的连续位置。
 */
function messageTimePosition(message: LaneMessage, maxTime: number): number {
  const duration = Math.max(0, (message.endAt ?? message.at) - message.at);
  const logicalTime = duration > 0 ? message.at + duration * clamp01(message.process?.progress ?? 1) : message.at;
  return Math.min(92, Math.max(0, (logicalTime / maxTime) * 88));
}

/**
 * clamp01 约束过程进度,防止仿真包错误数据撑破渲染布局。
 */
function clamp01(value: number): number {
  return Math.min(1, Math.max(0, Number.isFinite(value) ? value : 0));
}

/**
 * 渲染单个区块卡片,用于主链和分叉复用。
 */
function ChainBlockCard({
  block,
  selectedElementId,
  onSelectElement,
  canonicalTip,
}: {
  block: ChainPattern['data']['blocks'][number];
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
  canonicalTip?: string;
}): React.ReactElement {
  return (
    <article
      className={clsx('sim-chain__block', `is-${block.status}`, selectedElementId === block.id && 'is-selected', canonicalTip === block.id && 'is-tip')}
      role="listitem"
      {...selectableElementProps(block.id, onSelectElement, 'block')}
    >
      <span className="sim-chain__height">#{block.height}</span>
      <span className="sim-chain__label">{block.label}</span>
      <code className="sim-chain__hash">{block.hash.slice(0, 10)}</code>
      <small>父 {block.parentHash.slice(0, 8)}</small>
    </article>
  );
}

/**
 * 返回矩阵单元的可读状态标签。
 */
function matrixStatusLabel(status: MatrixPattern['data']['cells'][number][number]['status']): string {
  const labels = { empty: '未开始', pending: '进行中', yes: '通过', no: '拒绝', fault: '异常' };
  return labels[status];
}

/**
 * 将图表序列转换为 SVG polyline 点串。
 */
function chartPolyline(series: ChartSeries, minX: number, maxX: number, maxY: number): string {
  return series.points
    .slice(-16)
    .map((point) => {
      const plotted = chartPoint(point, minX, maxX, maxY);
      return `${plotted.x},${plotted.y}`;
    })
    .join(' ');
}

/**
 * 将数据点映射到图表逻辑坐标。
 */
function chartPoint(point: { x: number; y: number }, minX: number, maxX: number, maxY: number): { x: number; y: number } {
  const x = 8 + ((point.x - minX) / Math.max(1, maxX - minX)) * 88;
  const y = 52 - (point.y / Math.max(1, maxY)) * 46;
  return { x, y };
}

/**
 * 让渲染器内的语义元素可以被键盘和鼠标选中,供 select-element 交互传递目标。
 */
function selectableElementProps(elementId: string, onSelectElement?: (elementId: string, elementType?: string) => void, elementType?: string) {
  if (!onSelectElement) {
    return {};
  }
  return {
    role: 'button',
    tabIndex: 0,
    onClick: () => onSelectElement(elementId, elementType),
    onKeyDown: (event: React.KeyboardEvent) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        onSelectElement(elementId, elementType);
      }
    },
  };
}
