/**
 * Badge / Dot:语义色徽标与小状态点(§5.1)。
 * Badge 用语义色对(浅底 + 深字 + 同族描边)标注状态/类别;jade/cinnabar 为品牌向色族。
 * 深色面板(沉浸式工作台,§7.1)传 onDark,改用墨底描边 + 墨底语义文字色。
 * Dot 是最小状态点,label 必填 —— 颜色不是唯一信息载体(§3 无障碍)。
 */
import type { ReactNode } from "react";
import { cva } from "class-variance-authority";
import { cn } from "../../lib/cn";

export type BadgeTone =
  | "neutral"
  | "success"
  | "warning"
  | "danger"
  | "info"
  | "jade"
  | "cinnabar";

const badgeVariants = cva(
  // whitespace-nowrap:标签是一个语义整体,窄列里由列宽让位,不把短标签竖排成一列字(规范 §5.1)
  "inline-flex items-center whitespace-nowrap rounded-full border px-2.5 py-0.5 text-xs font-medium",
  {
    variants: {
      tone: {
        neutral: "bg-surface-sunken text-ink-sub border-line",
        success: "bg-success-bg text-success border-success-border",
        warning: "bg-warning-bg text-warning border-warning-border",
        danger: "bg-danger-bg text-danger border-danger-border",
        info: "bg-info-bg text-info border-info-border",
        jade: "bg-primary-soft text-primary border-transparent",
        cinnabar: "bg-seal-soft text-cinnabar-600 border-transparent",
      },
      // 深色面板(沉浸式工作台,§7.1):浅底徽标在墨底上是一块刺眼的亮片,
      // 改为墨底描边 + 墨底语义文字色。语义色族保持同名,信息不变、只换语境。
      onDark: { true: "", false: "" },
    },
    compoundVariants: [
      { onDark: true, tone: "neutral", class: "bg-dark-elevated text-on-dark-sub border-dark-line" },
      { onDark: true, tone: "success", class: "bg-dark-elevated text-accent border-accent" },
      { onDark: true, tone: "warning", class: "bg-dark-elevated text-on-dark-amber border-on-dark-amber" },
      { onDark: true, tone: "danger", class: "bg-dark-elevated text-on-dark-danger border-on-dark-danger" },
      { onDark: true, tone: "info", class: "bg-dark-elevated text-on-dark-blue border-on-dark-blue" },
      { onDark: true, tone: "jade", class: "bg-dark-elevated text-accent border-accent" },
      { onDark: true, tone: "cinnabar", class: "bg-dark-elevated text-energy border-energy" },
    ],
    defaultVariants: {
      tone: "neutral",
      onDark: false,
    },
  },
);

export interface BadgeProps {
  /** 语义色族;jade/cinnabar 为品牌向,danger 才是错误(朱砂不用于错误) */
  tone?: BadgeTone;
  /** 深色面板语境(沉浸式工作台):改用墨底语义令牌 */
  onDark?: boolean;
  children: ReactNode;
  className?: string;
}

export function Badge({ tone = "neutral", onDark = false, children, className }: BadgeProps) {
  return <span className={cn(badgeVariants({ tone, onDark }), className)}>{children}</span>;
}

/** 状态点的色映射:点本身仅是视觉强化,信息由 label 文字承载 */
const DOT_TONE: Record<BadgeTone, string> = {
  neutral: "bg-ink-faint",
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-danger",
  info: "bg-info",
  jade: "bg-jade-500",
  cinnabar: "bg-cinnabar-500",
};

export interface DotProps {
  /** 语义色族 */
  tone?: BadgeTone;
  /** 必填:状态文字,保证色盲/读屏用户也能获取状态 */
  label: string;
  className?: string;
}

export function Dot({ tone = "neutral", label, className }: DotProps) {
  return (
    <span className={cn("inline-flex items-center gap-1.5 text-sm text-ink-sub", className)}>
      <span aria-hidden="true" className={cn("size-2 shrink-0 rounded-full", DOT_TONE[tone])} />
      {label}
    </span>
  );
}
