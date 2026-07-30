/**
 * StatusIndicator:运行/流程状态指示(§5.1)。
 * 三种形态:soft(浅底胶囊)/ solid(实底胶囊)/ text(纯文字级);label 必填,
 * 颜色不是唯一信息载体(§3 无障碍)。loading 时用 LoaderCircle 旋转 —— 加载指示允许旋转;
 * 规范红线:禁止任何无限脉冲/呼吸类装饰动画(§4.1),状态常态必须是静止的。
 */
import type { LucideIcon } from "lucide-react";
import { LoaderCircle } from "lucide-react";
import { cva } from "class-variance-authority";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export type StatusTone = "neutral" | "primary" | "success" | "warning" | "danger" | "info";
export type StatusAppearance = "soft" | "solid" | "text";

const statusVariants = cva("inline-flex items-center gap-1.5 text-xs font-medium", {
  variants: {
    appearance: {
      soft: "rounded-full border px-2.5 py-0.5",
      solid: "rounded-full border border-transparent px-2.5 py-0.5",
      text: "",
    },
    tone: {
      neutral: "",
      primary: "",
      success: "",
      warning: "",
      danger: "",
      info: "",
    },
  },
  // tone × appearance 的色对全部枚举,保证每个组合都经过对比度校验的令牌
  compoundVariants: [
    { appearance: "soft", tone: "neutral", class: "bg-surface-sunken text-ink-sub border-line" },
    { appearance: "soft", tone: "primary", class: "bg-primary-soft text-primary border-transparent" },
    { appearance: "soft", tone: "success", class: "bg-success-bg text-success border-success-border" },
    { appearance: "soft", tone: "warning", class: "bg-warning-bg text-warning border-warning-border" },
    { appearance: "soft", tone: "danger", class: "bg-danger-bg text-danger border-danger-border" },
    { appearance: "soft", tone: "info", class: "bg-info-bg text-info border-info-border" },
    // solid 一律用 on-solid(纯白):各语义实底组合均 ≥4.5:1(warning 上 on-dark 仅 4.4,不达标)
    { appearance: "solid", tone: "neutral", class: "bg-ink-800 text-on-solid" },
    { appearance: "solid", tone: "primary", class: "bg-primary text-on-solid" },
    { appearance: "solid", tone: "success", class: "bg-success text-on-solid" },
    { appearance: "solid", tone: "warning", class: "bg-warning text-on-solid" },
    { appearance: "solid", tone: "danger", class: "bg-danger text-on-solid" },
    { appearance: "solid", tone: "info", class: "bg-info text-on-solid" },
    { appearance: "text", tone: "neutral", class: "text-ink-sub" },
    { appearance: "text", tone: "primary", class: "text-primary" },
    { appearance: "text", tone: "success", class: "text-success" },
    { appearance: "text", tone: "warning", class: "text-warning" },
    { appearance: "text", tone: "danger", class: "text-danger" },
    { appearance: "text", tone: "info", class: "text-info" },
  ],
  defaultVariants: {
    appearance: "soft",
    tone: "neutral",
  },
});

export interface StatusIndicatorProps {
  /** 语义色族 */
  tone: StatusTone;
  /** 形态:soft 浅底胶囊(默认)/ solid 实底胶囊 / text 纯文字级 */
  appearance?: StatusAppearance;
  /** 必填:状态文字,色非唯一信息载体 */
  label: string;
  /** 状态图标(loading 时被旋转指示替代) */
  icon?: LucideIcon;
  /** 进行中:显示旋转加载指示(唯一允许的循环动画) */
  loading?: boolean;
  className?: string;
}

export function StatusIndicator({
  tone,
  appearance = "soft",
  label,
  icon,
  loading = false,
  className,
}: StatusIndicatorProps) {
  return (
    <span className={cn(statusVariants({ appearance, tone }), className)}>
      {loading ? (
        <Icon icon={LoaderCircle} size="sm" className="animate-spin" />
      ) : (
        icon && <Icon icon={icon} size="sm" />
      )}
      {label}
    </span>
  );
}
