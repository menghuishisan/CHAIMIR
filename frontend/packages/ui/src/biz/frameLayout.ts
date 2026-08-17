/**
 * frameLayout:把 TeachingFrame.layout 解析成三区骨架各自要渲染的内容(§7.1 主次结构)。
 * 只做分区与归位:主舞台取 layout.primary,左辅助区按 evidence → metrics → trace → checkpoints
 * 顺序取其余模式(跨区去重、跳过主舞台),右事件流取无界元素(见 frameStream),
 * 标注按 target 归到拥有该编号的模式上。
 * 分区规则集中在此,各视图组件对同一帧得出一致结论。
 */
import { patternElementIds } from "./frameVisual";
import type { FrameAnnotation, PatternBinding, TeachingFrame } from "@chaimir/sim-sdk";

/** 辅助区的一块面板:一个模式 + 它在本帧承担的教学职责 */
export interface AsidePanel {
  /** 职责名(用户向,如「证据」「时间线」) */
  region: string;
  pattern: PatternBinding;
}

/** 标注归位结果:按模式编号分组 + 未指向本帧任何模式/元素的标注 */
export interface AnnotationPartition {
  byPattern: Map<string, FrameAnnotation[]>;
  /** 未命中的标注由主舞台统一列出,不静默丢弃 */
  unassigned: FrameAnnotation[];
}

/**
 * 辅助区职责的固定顺序与用户向名称:证据先于指标,追踪与检查点收尾。
 * 不含 timeline:消息/调用时序即右侧事件流(§7.1 状态与事件分流),
 * 在辅助区再放一份泳道等于同一批消息出现两遍。
 */
function regionEntries(layout: TeachingFrame["layout"]): Array<[string[], string]> {
  return [
    [layout.evidence ?? [], "证据"],
    [layout.metrics ?? [], "指标"],
    [layout.trace ? [layout.trace] : [], "执行追踪"],
    [layout.checkpoints ?? [], "检查点"],
  ];
}

/** patternIndex:模式编号 → 模式,供 layout 各区按编号取回本体 */
function patternIndex(frame: TeachingFrame): Map<string, PatternBinding> {
  return new Map(frame.patterns.map((pattern) => [pattern.id, pattern]));
}

/**
 * pickPatterns:按编号列表取回模式本体。
 * `validateFrame`(sim-sdk)已双向校验 layout 引用与 patterns 声明,取不到即协议非法、
 * 该帧不会进入渲染;此处的类型收窄只为满足 Map 的可空返回,不承担纠错职责。
 */
export function pickPatterns(frame: TeachingFrame, ids: string[]): PatternBinding[] {
  const byId = patternIndex(frame);
  return ids
    .map((id) => byId.get(id))
    .filter((pattern): pattern is PatternBinding => pattern !== undefined);
}

/**
 * asidePanels:左辅助区面板序列。
 * 同一模式可被多个区同时引用(如既是 primary 又是 evidence),此处按首次出现的职责收敛一次;
 * 主舞台已渲染的模式、以及被声明为 timeline 的模式(那是右侧事件流)都不在辅助区重复出现。
 */
export function asidePanels(frame: TeachingFrame): AsidePanel[] {
  const byId = patternIndex(frame);
  const used = new Set<string>([frame.layout.primary]);
  if (frame.layout.timeline) used.add(frame.layout.timeline);
  const panels: AsidePanel[] = [];
  for (const [ids, region] of regionEntries(frame.layout)) {
    for (const id of ids) {
      if (used.has(id)) continue;
      used.add(id);
      const pattern = byId.get(id);
      if (pattern) panels.push({ region, pattern });
    }
  }
  return panels;
}

/** frameHasAside:本帧是否有辅助区内容,供页面决定是否挂出右栏(避免空面板外壳) */
export function frameHasAside(frame: TeachingFrame): boolean {
  return asidePanels(frame).length > 0;
}

/**
 * partitionAnnotations:标注按 target 归位。
 * target 可以是模式编号,也可以是模式内某个元素编号(节点/边/区块/单元格/步骤/消息/序列);
 * 归到拥有该编号的模式上,由该模式所在分区展示,未命中的交给主舞台列出。
 */
export function partitionAnnotations(frame: TeachingFrame): AnnotationPartition {
  // 元素编号 → 所属模式编号;模式自身编号也可作为 target
  const owners = new Map<string, string>();
  for (const pattern of frame.patterns) {
    owners.set(pattern.id, pattern.id);
    for (const elementId of patternElementIds(pattern)) {
      if (!owners.has(elementId)) owners.set(elementId, pattern.id);
    }
  }

  const byPattern = new Map<string, FrameAnnotation[]>();
  const unassigned: FrameAnnotation[] = [];
  for (const annotation of frame.annotations ?? []) {
    const owner = owners.get(annotation.target);
    if (owner === undefined) {
      unassigned.push(annotation);
      continue;
    }
    const list = byPattern.get(owner);
    if (list) list.push(annotation);
    else byPattern.set(owner, [annotation]);
  }
  return { byPattern, unassigned };
}
