/**
 * TopologyGraph:节点关系拓扑图(规范 §8.1「节点与边的关系」)。
 *
 * 用途是回答「谁连着谁、谁掉了」。这类数据的形状本身是图而不是序列或类别,
 * 摊成表格后读者要在脑子里重建连接关系 —— 竞赛攻防回放、共识节点连通、
 * 沙箱网络策略都属于这一档。
 *
 * **实现用内联 SVG。** Recharts 没有拓扑图元,而这里既不需要坐标系也不需要比例尺,
 * 为一张图引第二个图表库不合算(CLAUDE.md §4:自研需说明原因与取舍)。
 *
 * 状态一律**形状 + 文字**双载体(§8.2 色非唯一):主节点是双环、落后节点带三角徽标、
 * 隔离节点是空心圆加叉,非正常节点还会在下方标出状态文字。只变颜色时色盲用户看不到任何差别。
 * 应包在 ChartContainer 内使用。
 */
import { useMemo } from "react";
import { cn } from "../lib/cn";
import { useChartColors, type ChartContext } from "./palette";

/** 节点状态语义:决定颜色**与形状**,不只决定颜色 */
export type TopologyTone = "normal" | "primary" | "warning" | "danger";

export interface TopologyNode {
  id: string;
  /** 节点内的短名,如 `n1`;过长会溢出圆形,建议 ≤4 字符 */
  label: string;
  /** 状态语义,默认 normal */
  tone?: TopologyTone;
  /** 状态文字(用户向),如「主节点」「同步落后」「已隔离」;非 normal 时必填才算色非唯一 */
  statusLabel?: string;
  /** 归一化坐标(0–1);两者都不传时按圆周均分自动布点 */
  x?: number;
  y?: number;
}

export interface TopologyEdge {
  from: string;
  to: string;
  /** danger 表示链路异常/已断开:虚线 + 冷红,与实线正常边区分 */
  tone?: "normal" | "danger";
}

export interface TopologyGraphProps {
  nodes: TopologyNode[];
  edges: TopologyEdge[];
  /** 图表高度(px),默认 260 */
  height?: number;
  /** 配色语境:paper=宣纸光面(默认),dark=墨底沉浸舞台/深色面板 */
  context?: ChartContext;
  className?: string;
}

/** 节点数上界(规范 §8.1:超过约 20 个改邻接矩阵) */
const MAX_NODES = 20;
/** 画布坐标系:固定 viewBox 让字号与节点半径在任何宽度下比例一致 */
const VIEW_W = 400;
const VIEW_H = 240;
/** 内边距:给节点下方的状态文字留位置,否则贴边裁掉 */
const PAD_X = 42;
const PAD_TOP = 26;
const PAD_BOTTOM = 34;
const NODE_R = 15;

export function TopologyGraph({
  nodes,
  edges,
  height = 260,
  context = "paper",
  className,
}: TopologyGraphProps) {
  const colors = useChartColors(context);

  if (nodes.length === 0) {
    throw new Error("TopologyGraph 至少需要 1 个节点:空拓扑应由 ChartContainer 的空态承担。");
  }
  if (nodes.length > MAX_NODES) {
    throw new Error(
      `TopologyGraph 最多 ${MAX_NODES} 个节点,收到 ${nodes.length} 个:` +
        `再多边会互相压住看不清连接关系(规范 §8.1),请改用邻接矩阵(DensityMatrix)。`,
    );
  }

  /**
   * 布点:显式给了归一化坐标就用它,否则按圆周均分。
   * 均分是确定性的 —— 力导向布局每次刷新都换位置,读者会以为拓扑变了。
   */
  const placed = useMemo(() => {
    const cx = VIEW_W / 2;
    const cy = (VIEW_H - PAD_TOP - PAD_BOTTOM) / 2 + PAD_TOP;
    const radiusX = (VIEW_W - PAD_X * 2) / 2;
    const radiusY = (VIEW_H - PAD_TOP - PAD_BOTTOM) / 2;
    return nodes.map((node, index) => {
      if (node.x !== undefined && node.y !== undefined) {
        return {
          node,
          cx: PAD_X + node.x * (VIEW_W - PAD_X * 2),
          cy: PAD_TOP + node.y * (VIEW_H - PAD_TOP - PAD_BOTTOM),
        };
      }
      // 单节点居中;多节点从 12 点起顺时针均分
      if (nodes.length === 1) return { node, cx, cy };
      const angle = -Math.PI / 2 + (index * 2 * Math.PI) / nodes.length;
      return { node, cx: cx + radiusX * Math.cos(angle), cy: cy + radiusY * Math.sin(angle) };
    });
  }, [nodes]);

  /** id → 坐标,画边时按 id 取两端 */
  const positionOf = useMemo(
    () => new Map(placed.map((item) => [item.node.id, { cx: item.cx, cy: item.cy }])),
    [placed],
  );

  /** tone → 填充色。normal 用玉深、primary 用玉亮、warning 琥珀、danger 冷红 */
  const fillOf = (tone: TopologyTone) =>
    tone === "warning" ? colors.warning : tone === "danger" ? colors.danger : colors.jade;

  /**
   * 边的两端必须都在 nodes 里。端点缺失是**数据一致性问题**而不是编程错误
   * (edges 通常来自服务端推送),所以按 CLAUDE.md §8 走「记日志后转可恢复路径」:
   * 结构化告警一条 + 跳过这条边,不吞错也不让整页白屏。
   */
  const drawableEdges = useMemo(
    () =>
      edges.filter((edge) => {
        const ok = positionOf.has(edge.from) && positionOf.has(edge.to);
        if (!ok) {
          console.warn("topology_edge_endpoint_missing", {
            operation: "render_topology_graph",
            from: edge.from,
            to: edge.to,
            knownNodeCount: positionOf.size,
          });
        }
        return ok;
      }),
    [edges, positionOf],
  );

  return (
    <div className={cn("min-w-0", className)}>
      <svg
        viewBox={`0 0 ${VIEW_W} ${VIEW_H}`}
        width="100%"
        height={height}
        preserveAspectRatio="xMidYMid meet"
        role="img"
        aria-label={`拓扑图,共 ${nodes.length} 个节点、${drawableEdges.length} 条连接`}
      >
        {/* 边先画,压在节点下面 */}
        {drawableEdges.map((edge) => {
          // 端点已在 drawableEdges 里校验过,这里必然取到
          const from = positionOf.get(edge.from)!;
          const to = positionOf.get(edge.to)!;
          const isDanger = edge.tone === "danger";
          return (
            <line
              key={`${edge.from}-${edge.to}`}
              x1={from.cx}
              y1={from.cy}
              x2={to.cx}
              y2={to.cy}
              stroke={isDanger ? colors.danger : colors.grid}
              strokeWidth={1.6}
              // 异常链路用虚线:线型是形状载体,不只靠变红(§8.2)
              strokeDasharray={isDanger ? "4 4" : undefined}
            />
          );
        })}

        {placed.map(({ node, cx, cy }) => {
          const tone = node.tone ?? "normal";
          const fill = fillOf(tone);
          const isolated = tone === "danger";
          return (
            <g key={node.id}>
              {/* title 必须是单个字符串:多个子节点会被 React 拒绝(浏览器只把 title 当纯文本) */}
              <title>{node.statusLabel ? `${node.label} · ${node.statusLabel}` : node.label}</title>

              {/* danger:空心圆 + 叉。形状本身就说明「这个节点不在网里」 */}
              {isolated ? (
                <>
                  <circle cx={cx} cy={cy} r={NODE_R} fill="none" stroke={fill} strokeWidth={2} />
                  <path
                    d={`M${cx - 5} ${cy - 5} l10 10 M${cx + 5} ${cy - 5} l-10 10`}
                    stroke={fill}
                    strokeWidth={2}
                    fill="none"
                  />
                </>
              ) : (
                <circle cx={cx} cy={cy} r={NODE_R} fill={fill} />
              )}

              {/* primary:再套一圈外环 —— 双环是「主节点」的形状标记 */}
              {tone === "primary" && (
                <circle cx={cx} cy={cy} r={NODE_R + 4.5} fill="none" stroke={fill} strokeWidth={2} />
              )}

              {/* warning:右上角三角徽标 —— 与圆形节点形状可辨 */}
              {tone === "warning" && (
                <path
                  d={`M${cx + NODE_R - 2} ${cy - NODE_R - 4} l6 10 h-12 Z`}
                  fill={fill}
                />
              )}

              {/* 节点内短名:非空心节点用反白字,空心节点用节点色 */}
              <text
                x={cx}
                y={cy + 3.5}
                textAnchor="middle"
                fontSize={10}
                fontWeight={600}
                fill={isolated ? fill : colors.surface}
              >
                {node.label}
              </text>

              {/* 状态文字:色非唯一的文字载体,只给非正常节点标,避免每个点都挂一行字 */}
              {tone !== "normal" && node.statusLabel && (
                <text
                  x={cx}
                  y={cy + NODE_R + 13}
                  textAnchor="middle"
                  fontSize={10}
                  fill={tone === "primary" ? colors.jade : fill}
                >
                  {node.statusLabel}
                </text>
              )}
            </g>
          );
        })}
      </svg>
    </div>
  );
}
