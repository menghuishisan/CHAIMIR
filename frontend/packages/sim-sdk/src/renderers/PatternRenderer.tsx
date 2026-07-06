// 本文件将仿真包输出的封闭可视化模式语义数据渲染为统一界面,避免各仿真包自带 DOM 或自定义渲染器。

import React from 'react';
import { clsx } from 'clsx';
import { BarChart3, GitBranch, Layers3, Network, Rows3, Table2, Workflow } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { triggerHaptic } from '@chaimir/ui';
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
  reducedMotion?: boolean;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}

/**
 * 按模式类型分发到平台维护的统一渲染器。
 */
export function PatternRenderer({ pattern, selectedElementId, reducedMotion = false, onSelectElement }: PatternRendererProps): React.ReactElement {
  switch (pattern.mode) {
    case 'graph':
      return <GraphRenderer pattern={pattern} selectedElementId={selectedElementId} reducedMotion={reducedMotion} onSelectElement={onSelectElement} />;
    case 'chain':
      return <ChainRenderer pattern={pattern} selectedElementId={selectedElementId} onSelectElement={onSelectElement} />;
    case 'tree':
      return <TreeRenderer pattern={pattern} selectedElementId={selectedElementId} onSelectElement={onSelectElement} />;
    case 'matrix':
      return <MatrixRenderer pattern={pattern} selectedElementId={selectedElementId} onSelectElement={onSelectElement} />;
    case 'pipeline':
      return <PipelineRenderer pattern={pattern} selectedElementId={selectedElementId} reducedMotion={reducedMotion} onSelectElement={onSelectElement} />;
    case 'lane':
      return <LaneRenderer pattern={pattern} selectedElementId={selectedElementId} reducedMotion={reducedMotion} onSelectElement={onSelectElement} />;
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
  reducedMotion,
  onSelectElement,
}: {
  pattern: GraphPattern;
  selectedElementId?: string;
  reducedMotion: boolean;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const nodes = layoutGraphNodes(pattern.data.nodes, pattern.data.layout);
  const nodeById = new Map(nodes.map((node) => [node.id, node]));
  const activeEdges = pattern.data.edges.filter((edge) => edge.status === 'active' || edge.status === 'success').length;
  const failedEdges = pattern.data.edges.filter((edge) => edge.status === 'failed').length;
  const riskyNodes = pattern.data.nodes.filter((node) => node.status === 'danger' || node.status === 'warning').length;
  const showEdgeLabels = pattern.data.edges.length <= 10;
  const markerBaseId = makeSvgId(pattern.id, 'edge-marker');

  return (
    <section className="sim-pattern sim-pattern--graph" aria-label={pattern.title}>
      <PatternHeader mode="graph" title={pattern.title} meta={`${pattern.data.nodes.length} 节点 / ${activeEdges} 活跃 / ${failedEdges} 异常`} />
      <PatternInsight items={[['风险节点', riskyNodes], ['消息总数', pattern.data.edges.length]]} />

      <div className="sim-graph-container" role="figure" aria-label={`${pattern.title}图网络`}>
        <div className="sim-graph__canvas">
          <svg className="sim-graph__svg-layer" viewBox="0 0 100 100">
            <defs>
              <marker className="sim-graph__marker is-muted" id={`${markerBaseId}-muted`} markerHeight="5" markerUnits="strokeWidth" markerWidth="5" orient="auto" refX="4.5" refY="2.5">
                <path d="M0,0 L5,2.5 L0,5 Z" />
              </marker>
              <marker className="sim-graph__marker is-accent" id={`${markerBaseId}-accent`} markerHeight="5" markerUnits="strokeWidth" markerWidth="5" orient="auto" refX="4.5" refY="2.5">
                <path d="M0,0 L5,2.5 L0,5 Z" />
              </marker>
              <marker className="sim-graph__marker is-danger" id={`${markerBaseId}-danger`} markerHeight="5" markerUnits="strokeWidth" markerWidth="5" orient="auto" refX="4.5" refY="2.5">
                <path d="M0,0 L5,2.5 L0,5 Z" />
              </marker>
            </defs>
            {pattern.data.edges.map((edge) => {
              const from = nodeById.get(edge.from);
              const to = nodeById.get(edge.to);
              if (!from || !to) return null;
              const line = shortenLine(from, to, 6.5);
              const progress = processProgress(edge, reducedMotion);
              const pulse = pointOnLine(line, progress);
              return (
                <g className={clsx('sim-graph__edge-group', selectedElementId === edge.id && 'is-selected')} key={edge.id} {...selectableElementProps(edge.id, onSelectElement, 'edge')}>
                  <line
                    className={clsx('sim-graph__edge', `is-${edge.status}`)}
                    x1={line.x1}
                    y1={line.y1}
                    x2={line.x2}
                    y2={line.y2}
                    markerEnd={`url(#${markerBaseId}-${graphMarkerTone(edge.status)})`}
                  />
                  <circle className={clsx('sim-graph__pulse', `is-${edge.status}`)} cx={pulse.x} cy={pulse.y} r="1.3">
                    <title>{edge.process?.label ?? edge.detail ?? edge.label}</title>
                  </circle>
                  {showEdgeLabels && (
                    <text className="sim-graph__edge-label" x={(from.x + to.x) / 2} y={(from.y + to.y) / 2}>
                      {edge.label}
                    </text>
                  )}
                </g>
              );
            })}
          </svg>

          <div className="sim-graph__nodes-layer">
            {nodes.map((node) => (
              <div
                key={node.id}
                className={clsx('sim-graph__node', `is-${node.status}`, selectedElementId === node.id && 'is-selected')}
                style={{ left: `${node.x}%`, top: `${node.y}%` }}
                {...selectableElementProps(node.id, onSelectElement, node.role)}
              >
                <div className="sim-graph__node-glass">
                  <span>{node.label}</span>
                  {node.value && <strong>{node.value}</strong>}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
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
  const attackerBlocks = forkBlocks.concat(canonical).filter((block) => block.status === 'attacker').length;
  return (
    <section className="sim-pattern sim-pattern--chain" aria-label={pattern.title}>
      <PatternHeader mode="chain" title={pattern.title} meta={`${canonical.length} 个主链块 / ${forkBlocks.length} 个分叉块`} />
      <PatternInsight items={[['规范链尖', pattern.data.canonicalTip ?? '等待'], ['攻击块', attackerBlocks]]} />
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
  const nodeCount = countTreeNodes(pattern.data.root);
  return (
    <section className="sim-pattern sim-pattern--tree" aria-label={pattern.title}>
      <PatternHeader mode="tree" title={pattern.title} meta={`${nodeCount} 节点 / 路径 ${pattern.data.highlightedPath.length} 层`} />
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
  const pathParent = node.children?.some((child) => treeContainsPath(child, path)) ?? false;
  return (
    <div className={clsx('sim-tree__node', isRoot && 'is-root', path.includes(node.id) && 'is-highlighted', pathParent && 'is-path-parent', selectedElementId === node.id && 'is-selected')}>
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
    <section className="sim-pattern sim-pattern--matrix" aria-label={pattern.title}>
      <PatternHeader mode="matrix" title={pattern.title} meta={`通过 ${statusCounts.yes ?? 0} / 处理中 ${statusCounts.pending ?? 0} / 异常 ${statusCounts.fault ?? 0}`} />
      <PatternInsight items={[['拒绝', statusCounts.no ?? 0], ['等待', statusCounts.empty ?? 0]]} />
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
  reducedMotion,
  onSelectElement,
}: {
  pattern: PipelinePattern;
  selectedElementId?: string;
  reducedMotion: boolean;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const complete = pattern.data.steps.filter((step) => step.status === 'complete').length;
  const running = pattern.data.steps.find((step) => step.status === 'running');
  return (
    <section className="sim-pattern sim-pattern--pipeline" aria-label={pattern.title}>
      <PatternHeader mode="pipeline" title={pattern.title} meta={`${complete}/${pattern.data.steps.length} 步完成`} />
      {running && <PatternInsight items={[['当前步骤', running.label], ['状态', pipelineStatusLabel(running.status)]]} />}
      <ol className="sim-pipeline">
        {pattern.data.steps.map((step, index) => (
          <li
            className={clsx('sim-pipeline__step', `is-${step.status}`, selectedElementId === step.id && 'is-selected')}
            key={step.id}
            aria-current={step.status === 'running' ? 'step' : undefined}
            {...selectableElementProps(step.id, onSelectElement, 'step')}
          >
            <i>{index + 1}</i>
            <span>{step.label}</span>
            <small>{step.detail}</small>
            {step.process && (
              <span className="sim-pipeline__progress" aria-label={`${step.process.label}${Math.round(step.process.progress * 100)}%`}>
                <span style={{ inlineSize: `${Math.round(processSpanProgress(step.process, reducedMotion) * 100)}%` }} />
                <em>{Math.round(processSpanProgress(step.process, reducedMotion) * 100)}%</em>
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
  reducedMotion,
  onSelectElement,
}: {
  pattern: LanePattern;
  selectedElementId?: string;
  reducedMotion: boolean;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const maxTime = Math.max(1, pattern.data.currentTime, ...pattern.data.messages.map((message) => message.endAt ?? message.at));
  const dropped = pattern.data.messages.filter((message) => message.status === 'dropped').length;
  const timeTicks = [0, Math.round(maxTime / 2), maxTime];
  return (
    <section className="sim-pattern sim-pattern--lane" aria-label={pattern.title}>
      <PatternHeader mode="lane" title={pattern.title} meta={`当前时间 ${pattern.data.currentTime} / 消息 ${pattern.data.messages.length}`} />
      <PatternInsight items={[['参与方', pattern.data.actors.length], ['丢弃', dropped]]} />
      <div className="sim-lane">
        <div className="sim-lane__axis" aria-hidden="true">
          <span />
          <div>
            {timeTicks.map((tick, index) => (
              <small key={`${tick}-${index}`} style={{ insetInlineStart: `${index === 0 ? 0 : index === 1 ? 50 : 100}%` }}>
                t{tick}
              </small>
            ))}
          </div>
        </div>
        {pattern.data.actors.map((actor) => (
          <div className="sim-lane__row" key={actor}>
            <span className="sim-lane__actor">{actor}</span>
            <div className="sim-lane__messages">
              {pattern.data.messages
                .filter((message) => message.from === actor || message.to === actor)
                .map((message) => {
                  const position = messageTimePosition(message, maxTime, reducedMotion);
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
                      {message.process && <i style={{ inlineSize: `${Math.round(processSpanProgress(message.process, reducedMotion) * 100)}%` }} aria-hidden="true" />}
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
  const ticks = chartTicks(maxY);
  const latest = pattern.data.series.map((series) => series.points[series.points.length - 1]?.y ?? 0);
  return (
    <section className="sim-pattern sim-pattern--chart" aria-label={pattern.title}>
      <PatternHeader mode="chart" title={pattern.title} meta={`最大值 ${maxY}${pattern.data.unit}`} />
      <PatternInsight items={[['序列', pattern.data.series.length], ['最新合计', latest.reduce((sum, value) => sum + value, 0)]]} />
      <div className="sim-chart">
        <svg className="sim-chart__plot" viewBox="0 0 100 60" role="img" aria-label={`${pattern.title}趋势图`}>
          <line className="sim-chart__axis" x1="8" y1="52" x2="96" y2="52" />
          <line className="sim-chart__axis" x1="8" y1="4" x2="8" y2="52" />
          {ticks.map((tick) => (
            <g key={tick}>
              <line className="sim-chart__grid" x1="8" x2="96" y1={chartY(tick, maxY)} y2={chartY(tick, maxY)} />
              <text className="sim-chart__tick" x="2" y={chartY(tick, maxY) + 2}>{tick}</text>
            </g>
          ))}
          <line className="sim-chart__current-marker" x1={chartPoint({ x: maxX, y: 0 }, minX, maxX, maxY).x} y1="4" x2={chartPoint({ x: maxX, y: 0 }, minX, maxX, maxY).x} y2="52" />
          {pattern.data.series.map((series, index) => {
            const visiblePoints = series.points.slice(-16);
            return (
              <g className={`sim-chart__series-line is-${index % 4}`} key={series.label}>
                <polyline points={chartPolyline(series, minX, maxX, maxY)} />
                {visiblePoints.map((point, pointIndex) => {
                  const pointId = `${pattern.id}:${series.label}:${point.x}`;
                  const plotted = chartPoint(point, minX, maxX, maxY);
                  return (
                    <circle
                      className={clsx(pointIndex === visiblePoints.length - 1 && 'is-latest', selectedElementId === pointId && 'is-selected')}
                      cx={plotted.x}
                      cy={plotted.y}
                      key={`${series.label}-${point.x}`}
                      r="1.7"
                      {...selectableElementProps(pointId, onSelectElement, 'point')}
                    />
                  );
                })}
              </g>
            );
          })}
        </svg>
        <div className="sim-chart__legend">
          {pattern.data.series.map((series, index) => (
            <span className={`is-${index % 4}`} key={series.label}>
              {series.label}
            </span>
          ))}
        </div>
        <table className="sim-chart__table">
          <caption>趋势数据</caption>
          <thead>
            <tr>
              <th scope="col">系列</th>
              <th scope="col">最新时间</th>
              <th scope="col">最新数值</th>
            </tr>
          </thead>
          <tbody>
            {pattern.data.series.map((series) => {
              const lastPoint = series.points[series.points.length - 1];
              return (
                <tr key={series.label}>
                  <th scope="row">{series.label}</th>
                  <td>{lastPoint?.x ?? 0}</td>
                  <td>{lastPoint ? `${lastPoint.y}${pattern.data.unit}` : `0${pattern.data.unit}`}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}

/**
 * 渲染统一模式标题和当前数据摘要。
 */
function PatternHeader({ mode, title, meta }: { mode: PatternBinding['mode']; title: string; meta: string }): React.ReactElement {
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
function PatternInsight({ items }: { items: Array<[string, string | number]> }): React.ReactElement {
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
function processProgress(edge: GraphEdge, reducedMotion: boolean): number {
  if (reducedMotion) {
    if (edge.status === 'pending') return 0.15;
    if (edge.status === 'active') return 0.5;
    return 0.85;
  }
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
 * countTreeNodes 统计树节点数量,用于 Merkle/Trie 类结构给出规模感。
 */
function countTreeNodes(node: TreeNode): number {
  return 1 + (node.children?.reduce((sum, child) => sum + countTreeNodes(child), 0) ?? 0);
}

/**
 * treeContainsPath 判断当前子树是否包含高亮路径节点,用于标出 proof path 的父链。
 */
function treeContainsPath(node: TreeNode, path: string[]): boolean {
  return path.includes(node.id) || Boolean(node.children?.some((child) => treeContainsPath(child, path)));
}

/**
 * messageTimePosition 将消息发送、到达和过程进度映射为泳道中的连续位置。
 */
function messageTimePosition(message: LaneMessage, maxTime: number, reducedMotion: boolean): number {
  const duration = Math.max(0, (message.endAt ?? message.at) - message.at);
  const progress = reducedMotion ? staticMessageProgress(message) : clamp01(message.process?.progress ?? 1);
  const logicalTime = duration > 0 ? message.at + duration * progress : message.at;
  return Math.min(92, Math.max(0, (logicalTime / maxTime) * 88));
}

/**
 * processSpanProgress 在减少动态时只保留离散状态,不播放连续过程。
 */
function processSpanProgress(process: { progress: number }, reducedMotion: boolean): number {
  return reducedMotion ? Math.round(clamp01(process.progress)) : clamp01(process.progress);
}

/**
 * staticMessageProgress 在减少动态时把泳道消息固定到起点或终点,避免滑行动效。
 */
function staticMessageProgress(message: LaneMessage): number {
  return message.status === 'sent' ? 0 : 1;
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
 * pipelineStatusLabel 返回流水线步骤的用户向状态名。
 */
function pipelineStatusLabel(status: PipelinePattern['data']['steps'][number]['status']): string {
  const labels = { pending: '等待', running: '运行中', complete: '完成', failed: '失败' };
  return labels[status];
}

/**
 * graphMarkerTone 将边状态映射到箭头标记,让消息方向与状态同时可见。
 */
function graphMarkerTone(status: GraphEdge['status']): 'accent' | 'danger' | 'muted' {
  if (status === 'failed') return 'danger';
  if (status === 'active' || status === 'success') return 'accent';
  return 'muted';
}

/**
 * makeSvgId 生成局部 SVG id,避免仿真包 id 中的特殊字符影响 marker 引用。
 */
function makeSvgId(...parts: string[]): string {
  return parts.join('-').replace(/[^a-zA-Z0-9_-]/g, '-');
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
  const y = chartY(point.y, maxY);
  return { x, y };
}

/**
 * chartTicks 根据真实最大值生成刻度,避免所有趋势图都显示固定 100 的误导性标尺。
 */
function chartTicks(maxY: number): number[] {
  const ceiling = Math.max(1, Math.ceil(maxY));
  return [0.25, 0.5, 0.75, 1].map((ratio) => Math.round(ceiling * ratio));
}

/**
 * chartY 将数值映射到图表坐标,所有趋势线和网格使用同一标尺。
 */
function chartY(value: number, maxY: number): number {
  return 52 - (value / Math.max(1, maxY)) * 46;
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
