// 本文件将仿真包输出的封闭可视化模式语义数据渲染为统一界面,避免各仿真包自带 DOM 或自定义渲染器。

import React from 'react';
import { clsx } from 'clsx';
import type {
  ChainPattern,
  ChartPattern,
  GraphPattern,
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
  const radius = 38;
  const center = 50;
  const nodes = pattern.data.nodes.map((node, index) => {
    const angle = (Math.PI * 2 * index) / Math.max(pattern.data.nodes.length, 1) - Math.PI / 2;
    return {
      ...node,
      x: center + Math.cos(angle) * radius,
      y: center + Math.sin(angle) * radius,
    };
  });
  const nodeById = new Map(nodes.map((node) => [node.id, node]));

  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <header className="sim-pattern__header">{pattern.title}</header>
      <svg className="sim-graph" viewBox="0 0 100 100" role="img" aria-label={`${pattern.title}图网络`}>
        {pattern.data.edges.map((edge) => {
          const from = nodeById.get(edge.from);
          const to = nodeById.get(edge.to);
          if (!from || !to) return null;
          return (
            <g key={edge.id}>
              <line
                className={clsx('sim-graph__edge', `is-${edge.status}`)}
                x1={from.x}
                y1={from.y}
                x2={to.x}
                y2={to.y}
              />
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
  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <header className="sim-pattern__header">{pattern.title}</header>
      <div className="sim-chain" role="list">
        {pattern.data.blocks.map((block) => (
          <article
            className={clsx('sim-chain__block', `is-${block.status}`, selectedElementId === block.id && 'is-selected')}
            key={block.id}
            role="listitem"
            {...selectableElementProps(block.id, onSelectElement, 'block')}
          >
            <span className="sim-chain__height">#{block.height}</span>
            <span className="sim-chain__label">{block.label}</span>
            <code className="sim-chain__hash">{block.hash.slice(0, 10)}</code>
          </article>
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
      <header className="sim-pattern__header">{pattern.title}</header>
      <div className="sim-tree">{renderTreeNode(pattern.data.root, pattern.data.highlightedPath, selectedElementId, onSelectElement)}</div>
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
  onSelectElement?: (elementId: string, elementType?: string) => void
): React.ReactElement {
  return (
    <div className={clsx('sim-tree__node', path.includes(node.id) && 'is-highlighted', selectedElementId === node.id && 'is-selected')}>
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
  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <header className="sim-pattern__header">{pattern.title}</header>
      <table className="sim-matrix">
        <thead>
          <tr>
            <th scope="col">节点</th>
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
                  {cell.label}
                </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>
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
  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <header className="sim-pattern__header">{pattern.title}</header>
      <ol className="sim-pipeline">
        {pattern.data.steps.map((step) => (
          <li
            className={clsx('sim-pipeline__step', `is-${step.status}`, selectedElementId === step.id && 'is-selected')}
            key={step.id}
            {...selectableElementProps(step.id, onSelectElement, 'step')}
          >
            <span>{step.label}</span>
            <small>{step.detail}</small>
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
  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <header className="sim-pattern__header">{pattern.title}</header>
      <div className="sim-lane">
        {pattern.data.actors.map((actor) => (
          <div className="sim-lane__row" key={actor}>
            <span className="sim-lane__actor">{actor}</span>
            <div className="sim-lane__messages">
              {pattern.data.messages
                .filter((message) => message.from === actor || message.to === actor)
                .map((message) => (
                  <span
                    className={clsx('sim-lane__message', `is-${message.status}`, selectedElementId === message.id && 'is-selected')}
                    key={`${actor}-${message.id}`}
                    {...selectableElementProps(message.id, onSelectElement, 'message')}
                  >
                    {message.label}
                  </span>
                ))}
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
  return (
    <section className="sim-pattern" aria-label={pattern.title}>
      <header className="sim-pattern__header">{pattern.title}</header>
      <div className="sim-chart">
        {pattern.data.series.map((series) => (
          <div className="sim-chart__series" key={series.label}>
            <span>{series.label}</span>
            <div className="sim-chart__bars">
              {series.points.slice(-16).map((point) => {
                const pointId = `${pattern.id}:${series.label}:${point.x}`;
                return (
                  <i
                    className={clsx(selectedElementId === pointId && 'is-selected')}
                    key={`${series.label}-${point.x}`}
                    style={{ height: `${Math.max(6, (point.y / maxY) * 100)}%` }}
                    {...selectableElementProps(pointId, onSelectElement, 'point')}
                  />
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
