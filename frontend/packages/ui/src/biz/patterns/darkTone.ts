/**
 * darkTone:墨底语境下的状态色阶与配套图标(七种模式共用一套映射)。
 * 语义:玉(accent)= 成功/达成,蓝 = 进行中,琥珀 = 需注意,冷红 = 异常,灰 = 未开始。
 * 每个色阶都配固定图标与状态文字,保证颜色不是唯一的状态表达(§8.2)。
 */
import { Activity, Circle, CircleCheck, CircleX, TriangleAlert, type LucideIcon } from "lucide-react";

export type DarkTone = "neutral" | "active" | "success" | "warning" | "danger";

/** 文字/图元色:SVG 图元统一用 currentColor + 这些类,避免第二份色板 */
export const TONE_TEXT: Record<DarkTone, string> = {
  neutral: "text-on-dark-sub",
  active: "text-on-dark-blue",
  success: "text-accent",
  warning: "text-on-dark-amber",
  danger: "text-on-dark-danger",
};

/** 描边色:卡片/单元格边框 */
export const TONE_BORDER: Record<DarkTone, string> = {
  neutral: "border-dark-line",
  active: "border-on-dark-blue",
  success: "border-accent",
  warning: "border-on-dark-amber",
  danger: "border-on-dark-danger",
};

/** 状态图标:形状区分,与色阶一一对应 */
export const TONE_ICON: Record<DarkTone, LucideIcon> = {
  neutral: Circle,
  active: Activity,
  success: CircleCheck,
  warning: TriangleAlert,
  danger: CircleX,
};

/** SVG 虚线样式:未开始/异常用虚线,进行中用长虚线,达成用实线(灰度下仍可区分) */
export const TONE_DASH: Record<DarkTone, string | undefined> = {
  neutral: "3 4",
  active: "8 4",
  success: undefined,
  warning: "6 3",
  danger: "2 3",
};
