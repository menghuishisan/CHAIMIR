/**
 * frameVisual:TeachingFrame 渲染的共享视觉与文案规则(墨底语境)。
 * 集中三件事:焦点/生命周期强调如何转成类名、过程进度如何归一、
 * 以及各模式如何枚举自身元素编号(供标注按目标归位)。
 * 集中在此的目的是让七种模式对同一份协议字段给出一致语义,不各写一套。
 */
import type { FrameFocus, PatternBinding, VisualElementMeta } from "@chaimir/sim-sdk";

/** 渲染密度:stage=宽主舞台,panel=约 288px 窄侧栏(两种尺寸都必须可读) */
export type PatternDensity = "stage" | "panel";

/** 元素强调档:与 VisualElementMeta.emphasis 同集合,由帧焦点覆盖 */
export type FrameEmphasis = VisualElementMeta["emphasis"];

/** 生命周期阶段的用户向说法(读屏与列表都用同一套词) */
export const LIFECYCLE_LABEL: Record<VisualElementMeta["lifecycle"]["state"], string> = {
  entering: "刚出现",
  active: "进行中",
  settled: "已稳定",
  leaving: "即将离开",
  archived: "已归档",
};

/** 强调档的用户向说法:焦点态需要文字说明,不能只靠亮度区分 */
export const EMPHASIS_LABEL: Record<FrameEmphasis, string> = {
  focus: "关注中",
  context: "相关",
  history: "历史",
  ghost: "已淡出",
};

/**
 * resolveEmphasis:元素/视图的最终强调档。
 * 帧焦点(focus)是当前画面的显式指挥,优先于元素自带的 emphasis;
 * 二者都没有声明时按「相关」呈现。
 */
export function resolveEmphasis(
  id: string,
  focus: FrameFocus,
  meta?: VisualElementMeta,
): FrameEmphasis {
  if (focus.primary.includes(id)) return "focus";
  if (focus.secondary?.includes(id)) return "context";
  if (focus.muted?.includes(id)) return "history";
  return meta?.emphasis ?? "context";
}

/** 强调档 → 容器类:历史/淡出靠降低不透明度后退,焦点靠玉色描边前置 */
export const EMPHASIS_BOX: Record<FrameEmphasis, string> = {
  focus: "border-accent",
  context: "border-dark-line",
  history: "border-dark-line opacity-70",
  ghost: "border-dark-line border-dashed opacity-40",
};

/** 强调档 → 画布图元类:SVG 同样只用令牌派生的 stroke/fill 与不透明度 */
export const EMPHASIS_MARK: Record<FrameEmphasis, string> = {
  focus: "opacity-100",
  context: "opacity-100",
  history: "opacity-70",
  ghost: "opacity-40",
};

/** clampProgress:把协议里的 0~1 进度收敛到可渲染百分比(整数) */
export function progressPercent(progress: number): number {
  return Math.round(Math.min(1, Math.max(0, progress)) * 100);
}

/**
 * shortLabel:画布上的短标签。
 * 画布空间有限(窄栏仅 288px),超长名称在图元上截断,完整名称一律在下方元素列表里给全,
 * 因此截断不会造成信息丢失。
 */
export function shortLabel(label: string, max: number): string {
  return label.length > max ? `${label.slice(0, max)}…` : label;
}

/**
 * patternElementIds:枚举一个视图内部所有可被标注引用的元素编号。
 * 标注(FrameAnnotation.target)既可指向视图本身,也可指向视图内某个元素,
 * 舞台据此把标注派发到对应视图,未命中的标注在舞台层统一列出(不静默丢弃)。
 */
export function patternElementIds(pattern: PatternBinding): string[] {
  switch (pattern.mode) {
    case "graph":
      return [...pattern.data.nodes.map((node) => node.id), ...pattern.data.edges.map((edge) => edge.id)];
    case "chain":
      return [...pattern.data.blocks, ...pattern.data.forks.flat()].map((block) => block.id);
    case "tree":
      return treeNodeIds(pattern.data.root);
    case "matrix":
      return matrixCellIds(pattern.data.rows, pattern.data.columns);
    case "pipeline":
      return pattern.data.steps.map((step) => step.id);
    case "lane":
      return pattern.data.messages.map((message) => message.id);
    case "chart":
      return pattern.data.series.map((series) => series.label);
  }
}

/** treeNodeIds:深度优先收集树节点编号 */
function treeNodeIds(node: { id: string; children?: Array<{ id: string; children?: unknown }> }): string[] {
  const ids = [node.id];
  for (const child of node.children ?? []) {
    ids.push(...treeNodeIds(child as Parameters<typeof treeNodeIds>[0]));
  }
  return ids;
}

/** matrixCellId:矩阵单元没有自带编号,按「行-列」合成稳定编号供标注引用 */
export function matrixCellId(row: string, column: string): string {
  return `${row}-${column}`;
}

/** matrixCellIds:按行列笛卡尔积生成全部单元编号 */
function matrixCellIds(rows: string[], columns: string[]): string[] {
  return rows.flatMap((row) => columns.map((column) => matrixCellId(row, column)));
}
