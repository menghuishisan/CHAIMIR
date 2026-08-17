/**
 * elementStatus:无界元素(图上的连接、泳道里的消息)的状态色阶与状态词。
 * 抽出来共用是因为同一条消息要在两处出现:画布上是一条边/一支箭头(图形),
 * 右侧事件流里是一行文本(§7.1 状态与事件分流)。两处必须给出同一个色阶与同一个状态词,
 * 否则学生会以为看到的是两件事。
 */
import type { DarkTone } from "./darkTone";
import type { GraphEdge, LaneMessage } from "@chaimir/sim-sdk";

/** 边状态 → 墨底色阶 */
export const EDGE_TONE: Record<GraphEdge["status"], DarkTone> = {
  pending: "neutral",
  active: "active",
  success: "success",
  failed: "danger",
};

/** 边状态词 */
export const EDGE_STATUS_TEXT: Record<GraphEdge["status"], string> = {
  pending: "等待发送",
  active: "传输中",
  success: "已送达",
  failed: "未送达",
};

/** 消息状态 → 墨底色阶 */
export const MESSAGE_TONE: Record<LaneMessage["status"], DarkTone> = {
  sent: "active",
  delivered: "success",
  dropped: "danger",
};

/** 消息状态词 */
export const MESSAGE_STATUS_TEXT: Record<LaneMessage["status"], string> = {
  sent: "已发出",
  delivered: "已送达",
  dropped: "已丢失",
};
