// GraphPatternRenderer 渲染节点、连线和消息过程。

import React from 'react';
import { clsx } from 'clsx';
import type { FrameFocus, GraphEdge, GraphNode, GraphPattern } from '../../types';
import { PatternHeader, PatternInsight, PatternLegend } from '../PatternChrome';
import { clamp01, elementVisualClasses, isActiveProcess, selectableElementProps } from '../patternUtils';
import './GraphPatternRenderer.css';

/**
 * 渲染节点、连线和消息流,用于共识、网络传播、状态机等图网络场景。
 */
export function GraphPatternRenderer({
  pattern,
  focus,
  selectedElementId,
  reducedMotion,
  onSelectElement,
}: {
  pattern: GraphPattern;
  focus?: FrameFocus;
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
              const pulse = progress === undefined ? undefined : pointOnLine(line, progress);
              return (
                <g className={clsx('sim-graph__edge-group', elementVisualClasses(edge, focus), selectedElementId === edge.id && 'is-selected')} key={edge.id} {...selectableElementProps(edge.id, onSelectElement, 'edge')}>
                  <line
                    className={clsx('sim-graph__edge', `is-${edge.status}`)}
                    x1={line.x1}
                    y1={line.y1}
                    x2={line.x2}
                    y2={line.y2}
                    markerEnd={`url(#${markerBaseId}-${graphMarkerTone(edge.status)})`}
                  />
                  {pulse && (
                    <circle className={clsx('sim-graph__pulse', `is-${edge.status}`)} cx={pulse.x} cy={pulse.y} r="1.3">
                      <title>{edge.process?.label ?? edge.detail ?? edge.label}</title>
                    </circle>
                  )}
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
                className={clsx('sim-graph__node', `is-${node.status}`, elementVisualClasses(node, focus), selectedElementId === node.id && 'is-selected')}
                style={{ left: `${node.x}%`, top: `${node.y}%` }}
                {...selectableElementProps(node.id, onSelectElement, node.role)}
              >
                <div className="sim-graph__node-surface">
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
 * processProgress 读取协议消息的过程进度,没有连续过程数据时按离散状态定位。
 */
function processProgress(edge: GraphEdge, reducedMotion: boolean): number | undefined {
  if (!edge.process || !isActiveProcess(edge.process)) return undefined;
  return reducedMotion ? Math.round(clamp01(edge.process.progress)) : clamp01(edge.process.progress);
}

/**
 * pointOnLine 将 0 到 1 的过程进度映射到 SVG 连线坐标。
 */
function pointOnLine(line: ReturnType<typeof shortenLine>, progress: number): { x: number; y: number } {
  return { x: line.x1 + (line.x2 - line.x1) * progress, y: line.y1 + (line.y2 - line.y1) * progress };
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
