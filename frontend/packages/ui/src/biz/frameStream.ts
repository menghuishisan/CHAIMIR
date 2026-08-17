/**
 * frameStream:把一帧里的「无界元素」抽成按时刻排序的事件流条目(规范 §7.1 状态与事件分流)。
 *
 * 无界元素 = 消息 / 调用 —— 数量随时间增长,永远不会收敛。它们一律进右侧事件流;
 * 有界元素(节点 / 区块 / 单元格 / 阶段步骤)留在主舞台。这条分界线是「主舞台永不滚动」的前提:
 * 一旦把增长型清单和图形塞进同一个滚动容器,消息一多图形就被顶出视口。
 *
 * 取数只看元素种类,不看 layout 声明的职责:泳道消息与图上的边都是消息,
 * 哪怕作者没声明 timeline 也要进流。同一条消息常被同一帧的图与泳道同时引用(编号相同),
 * 故按编号去重 —— 泳道条目优先,它带权威的发生时刻。
 */
import { resolveEmphasis } from "./frameVisual";
import { EDGE_STATUS_TEXT, EDGE_TONE, MESSAGE_STATUS_TEXT, MESSAGE_TONE } from "./patterns/elementStatus";
import type { ElementListItem } from "./patterns/ElementList";
import type { TeachingFrame } from "@chaimir/sim-sdk";

/** 事件流条目 = 元素清单行 + 发生时刻 + 是否属于「攻击与失败」 */
export interface FrameStreamEntry extends ElementListItem {
  /** 发生时刻(推演 tick),用于按时刻分组 */
  at: number;
  /** 攻击或失败:消息丢失 / 调用未送达,供「只看攻击与失败」筛选 */
  adverse: boolean;
}

/**
 * frameStreamEntries:抽取本帧的全部事件流条目,按时刻升序、同刻按出现顺序。
 * 返回空数组表示这一帧还没有任何消息 —— 由调用方决定空态文案。
 */
export function frameStreamEntries(frame: TeachingFrame): FrameStreamEntry[] {
  // 编号 → 条目;泳道后写覆盖图的同编号条目(泳道带准确的 at)
  const byId = new Map<string, FrameStreamEntry>();
  const order: string[] = [];

  const remember = (entry: FrameStreamEntry, override: boolean) => {
    if (!byId.has(entry.id)) order.push(entry.id);
    else if (!override) return;
    byId.set(entry.id, entry);
  };

  for (const pattern of frame.patterns) {
    if (pattern.mode === "graph") {
      const nodeLabel = (id: string) =>
        pattern.data.nodes.find((node) => node.id === id)?.label ?? id;
      for (const edge of pattern.data.edges) {
        remember(
          {
            id: edge.id,
            label: `${nodeLabel(edge.from)} → ${nodeLabel(edge.to)}:${edge.label}`,
            tone: EDGE_TONE[edge.status],
            statusText: EDGE_STATUS_TEXT[edge.status],
            detail: edge.detail ?? edge.meta?.explanation,
            emphasis: resolveEmphasis(edge.id, frame.focus, edge.meta),
            progress: edge.process?.progress,
            progressLabel: edge.process?.label,
            lifecycle: edge.meta?.lifecycle.state,
            // 边没有独立的时刻字段:优先取过程起点,其次取生命周期起点
            at: edge.process?.startedAt ?? edge.meta?.lifecycle.fromTick ?? 0,
            adverse: edge.status === "failed",
          },
          false,
        );
      }
      continue;
    }
    if (pattern.mode === "lane") {
      for (const message of pattern.data.messages) {
        remember(
          {
            id: message.id,
            label: `${message.from} → ${message.to}:${message.label}`,
            tone: MESSAGE_TONE[message.status],
            statusText: MESSAGE_STATUS_TEXT[message.status],
            detail: message.detail ?? message.meta?.explanation,
            emphasis: resolveEmphasis(message.id, frame.focus, message.meta),
            progress: message.process?.progress,
            progressLabel: message.process?.label,
            lifecycle: message.meta?.lifecycle.state,
            at: message.at,
            adverse: message.status === "dropped",
          },
          true,
        );
      }
    }
  }

  const entries = order.map((id) => byId.get(id)).filter((entry): entry is FrameStreamEntry => entry !== undefined);
  // 稳定排序:同一时刻内保留作者给出的先后
  return entries
    .map((entry, index) => ({ entry, index }))
    .sort((left, right) => left.entry.at - right.entry.at || left.index - right.index)
    .map((item) => item.entry);
}

/**
 * frameHasStream:本帧是否声明了可承载消息的模式(graph 的边 / lane 的消息)。
 * 页面据此决定要不要挂出事件流栏 —— 判据是「模式声明」而不是「当前有没有消息」:
 * 共识类场景开局那一刻消息为零,但它一定会有,栏位不能等到第一条消息才出现(布局会跳);
 * 而默克尔树、UTXO 集这类只有有界元素的场景永远不会有消息,给它留一条空栏是占着屏幕撒谎。
 */
export function frameHasStream(frame: TeachingFrame): boolean {
  return frame.patterns.some((pattern) => pattern.mode === "graph" || pattern.mode === "lane");
}
