/**
 * GraphPatternView:拓扑视图(graph 模式)。
 * 节点按 ring/grid/layered 三种布局定位,边按状态用不同虚线样式与箭头绘制,
 * 带 ProcessSpan 的边以进度点表达传输位置(静态位置,不做循环动画)。
 * 画布只负责形状,信息完整性由下方元素清单承担;窄栏下画布压缩但清单不变。
 */
import { useId } from "react";
import { cn } from "../../lib/cn";
import {
  EMPHASIS_MARK,
  progressPercent,
  resolveEmphasis,
  shortLabel,
} from "../frameVisual";
import { TONE_DASH, TONE_TEXT } from "./darkTone";
import type { DarkTone } from "./darkTone";
import { ElementList, type ElementListItem } from "./ElementList";
import type { PatternViewProps } from "./types";
import type { GraphEdge, GraphNode, GraphPattern } from "@chaimir/sim-sdk";

/** 节点状态 → 墨底色阶 */
const NODE_TONE: Record<GraphNode["status"], DarkTone> = {
  idle: "neutral",
  active: "active",
  success: "success",
  warning: "warning",
  danger: "danger",
};

/** 节点状态词:与色阶同源,读屏与清单共用 */
const NODE_STATUS_TEXT: Record<GraphNode["status"], string> = {
  idle: "待启动",
  active: "工作中",
  success: "已完成",
  warning: "需注意",
  danger: "异常",
};

/** 边状态 → 墨底色阶 */
const EDGE_TONE: Record<GraphEdge["status"], DarkTone> = {
  pending: "neutral",
  active: "active",
  success: "success",
  failed: "danger",
};

/** 边状态词 */
const EDGE_STATUS_TEXT: Record<GraphEdge["status"], string> = {
  pending: "等待发送",
  active: "传输中",
  success: "已送达",
  failed: "未送达",
};

/** 画布坐标系:固定 viewBox,靠 preserveAspectRatio 自适应容器宽度 */
const VIEW_W = 320;
const VIEW_H = 220;
const NODE_R = 22;

interface Point {
  x: number;
  y: number;
}

/**
 * nodePositions:按布局算出每个节点的坐标。
 * ring 均分圆周;grid 按接近正方的列数铺格;layered 按 role 分层(同 role 一层)。
 */
function nodePositions(layout: GraphPattern["data"]["layout"], nodes: GraphNode[]): Map<string, Point> {
  const positions = new Map<string, Point>();
  if (nodes.length === 0) return positions;

  if (layout === "ring") {
    const radius = Math.min(VIEW_W, VIEW_H) / 2 - NODE_R - 12;
    nodes.forEach((node, index) => {
      const angle = (index / nodes.length) * Math.PI * 2 - Math.PI / 2;
      positions.set(node.id, {
        x: VIEW_W / 2 + radius * Math.cos(angle),
        y: VIEW_H / 2 + radius * Math.sin(angle),
      });
    });
    return positions;
  }

  if (layout === "grid") {
    const columns = Math.ceil(Math.sqrt(nodes.length));
    const rows = Math.ceil(nodes.length / columns);
    nodes.forEach((node, index) => {
      const column = index % columns;
      const row = Math.floor(index / columns);
      positions.set(node.id, {
        x: ((column + 1) / (columns + 1)) * VIEW_W,
        y: ((row + 1) / (rows + 1)) * VIEW_H,
      });
    });
    return positions;
  }

  // layered:按 role 归层,层内水平均分,层间垂直均分
  const roles: string[] = [];
  for (const node of nodes) {
    if (!roles.includes(node.role)) roles.push(node.role);
  }
  for (const role of roles) {
    const layerNodes = nodes.filter((node) => node.role === role);
    const rowIndex = roles.indexOf(role);
    layerNodes.forEach((node, index) => {
      positions.set(node.id, {
        x: ((index + 1) / (layerNodes.length + 1)) * VIEW_W,
        y: ((rowIndex + 1) / (roles.length + 1)) * VIEW_H,
      });
    });
  }
  return positions;
}

/** pointOnEdge:按进度取边上的一点,用于画传输位置圆点(clamp 到 0~1) */
function pointOnEdge(from: Point, to: Point, progress: number): Point {
  const ratio = progressPercent(progress) / 100;
  return { x: from.x + (to.x - from.x) * ratio, y: from.y + (to.y - from.y) * ratio };
}

export function GraphPatternView({
  pattern,
  focus,
  density,
  selectedElementId,
  onSelectElement,
}: PatternViewProps<GraphPattern>) {
  // defs 内箭头 marker 的 id 必须页内唯一(同页可能有多张图)
  const markerBase = useId().replace(/:/g, "");
  const { layout, nodes, edges } = pattern.data;
  const positions = nodePositions(layout, nodes);
  const compact = density === "panel";

  const nodeItems: ElementListItem[] = nodes.map((node) => ({
    id: node.id,
    label: node.meta?.label ?? node.label,
    tone: NODE_TONE[node.status],
    statusText: NODE_STATUS_TEXT[node.status],
    detail: [node.role, node.value, node.meta?.explanation].filter(Boolean).join(" · ") || undefined,
    emphasis: resolveEmphasis(node.id, focus, node.meta),
    lifecycle: node.meta?.lifecycle.state,
  }));

  const edgeItems: ElementListItem[] = edges.map((edge) => {
    const fromLabel = nodes.find((node) => node.id === edge.from)?.label ?? edge.from;
    const toLabel = nodes.find((node) => node.id === edge.to)?.label ?? edge.to;
    return {
      id: edge.id,
      label: `${fromLabel} → ${toLabel}:${edge.label}`,
      tone: EDGE_TONE[edge.status],
      statusText: EDGE_STATUS_TEXT[edge.status],
      detail: edge.detail ?? edge.meta?.explanation,
      emphasis: resolveEmphasis(edge.id, focus, edge.meta),
      progress: edge.process?.progress,
      progressLabel: edge.process?.label,
      lifecycle: edge.meta?.lifecycle.state,
    };
  });

  return (
    <div className="flex flex-col gap-3">
      <svg
        viewBox={`0 0 ${VIEW_W} ${VIEW_H}`}
        preserveAspectRatio="xMidYMid meet"
        role="presentation"
        className={cn("w-full", compact ? "max-h-40" : "max-h-96")}
      >
        <defs>
          {/* 每个色阶一个箭头 marker:marker 不继承 currentColor,需按色阶预置 */}
          {(Object.keys(TONE_DASH) as DarkTone[]).map((tone) => (
            <marker
              key={tone}
              id={`${markerBase}-arrow-${tone}`}
              viewBox="0 0 8 8"
              refX={7}
              refY={4}
              markerWidth={5}
              markerHeight={5}
              orient="auto-start-reverse"
            >
              <path d="M0,0 L8,4 L0,8 z" className={cn("fill-current", TONE_TEXT[tone])} />
            </marker>
          ))}
        </defs>

        {/* 先画边,节点覆盖其上,避免连线压住节点文字 */}
        {edges.map((edge) => {
          const from = positions.get(edge.from);
          const to = positions.get(edge.to);
          if (!from || !to) return null;
          const tone = EDGE_TONE[edge.status];
          const emphasis = resolveEmphasis(edge.id, focus, edge.meta);
          const marker = `url(#${markerBase}-arrow-${tone})`;
          // 传输位置圆点:进度落点先算好,避免在 JSX 里重复计算
          const processPoint = edge.process ? pointOnEdge(from, to, edge.process.progress) : undefined;
          return (
            <g key={edge.id} className={cn(TONE_TEXT[tone], EMPHASIS_MARK[emphasis])}>
              <line
                x1={from.x}
                y1={from.y}
                x2={to.x}
                y2={to.y}
                stroke="currentColor"
                strokeWidth={emphasis === "focus" ? 2 : 1.2}
                strokeDasharray={TONE_DASH[tone]}
                markerEnd={marker}
              />
              {/* 传输进度:圆点落在边上的对应位置,位置即进度(静态) */}
              {processPoint && (
                <circle cx={processPoint.x} cy={processPoint.y} r={3.5} fill="currentColor" />
              )}
            </g>
          );
        })}

        {nodes.map((node) => {
          const point = positions.get(node.id);
          if (!point) return null;
          const tone = NODE_TONE[node.status];
          const emphasis = resolveEmphasis(node.id, focus, node.meta);
          const selected = node.id === selectedElementId;
          return (
            <g key={node.id} className={cn(TONE_TEXT[tone], EMPHASIS_MARK[emphasis])}>
              <circle
                cx={point.x}
                cy={point.y}
                r={NODE_R}
                className="fill-dark-elevated"
                stroke="currentColor"
                strokeWidth={selected || emphasis === "focus" ? 2.5 : 1.4}
                strokeDasharray={node.status === "danger" ? "4 3" : undefined}
              />
              <text
                x={point.x}
                y={point.y + 4}
                textAnchor="middle"
                fontSize={11}
                fill="currentColor"
              >
                {shortLabel(node.label, 5)}
              </text>
            </g>
          );
        })}
      </svg>

      <ElementList
        label={`${pattern.title} 节点`}
        items={nodeItems}
        selectedElementId={selectedElementId}
        onSelectElement={onSelectElement}
      />
      {edgeItems.length > 0 && (
        <ElementList
          label={`${pattern.title} 连接`}
          items={edgeItems}
          selectedElementId={selectedElementId}
          onSelectElement={onSelectElement}
        />
      )}
    </div>
  );
}
