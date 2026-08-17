/**
 * Stat:统计指标卡(§5.1 / §6.5 指标带)。
 * Display 字体大数字 + tabular-nums;可带 Lucide 图标、涨跌 delta、补充说明,
 * 以及迷你链进度(chain:数值下方渲染 sm 号 ChainProgress,§5.1「可带迷你链」)。
 * delta 色非唯一:方向永远配 TrendingUp/TrendingDown 图标,不依赖颜色传达。
 */
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { TrendingDown, TrendingUp } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";
import { ChainProgress } from "../ChainProgress";

export interface StatDelta {
  /** 变化量文字,如 "+12.5%"、"-3" */
  value: string;
  direction: "up" | "down";
  /** 语义倾向;缺省按方向推断(up=positive,down=negative)——涨未必好时显式传入 */
  tone?: "positive" | "negative" | "neutral";
}

export interface StatProps {
  /** 指标名称(用户向文案) */
  label: string;
  value: ReactNode;
  icon?: LucideIcon;
  delta?: StatDelta;
  /** 补充说明,如统计口径、对比周期 */
  hint?: string;
  /** 迷你链进度:传入时在数值下方渲染 sm 号 ChainProgress(如「已完成实验 3/5」) */
  chain?: { done: number; total: number };
  className?: string;
}

/** delta 语义倾向 → 文字色(方向另有图标,颜色仅为辅助) */
const DELTA_TONE_CLASS = {
  positive: "text-success",
  negative: "text-danger",
  neutral: "text-ink-sub",
} as const;

export function Stat({ label, value, icon, delta, hint, chain, className }: StatProps) {
  // 缺省语义:上升视为积极、下降视为消极;特殊指标(如错误率)由调用方显式传 tone
  const deltaTone = delta ? (delta.tone ?? (delta.direction === "up" ? "positive" : "negative")) : undefined;

  return (
    // 抬起片(规范 §6.5.1 第 1 级):与 Card 同一层语言,不画边框
    <div className={cn("rounded-lg bg-surface p-5 shadow-xs", className)}>
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm text-ink-sub">{label}</span>
        {icon && <Icon icon={icon} size="lg" className="text-ink-faint" />}
      </div>
      <div className="mt-2 font-display text-3xl text-ink tabular-nums">{value}</div>
      {/* 迷你链进度紧贴数值下方(§5.1),delta/hint 再排其后,层次:数值 > 链 > 补充 */}
      {chain && (
        <ChainProgress size="sm" done={chain.done} total={chain.total} className="mt-2" />
      )}
      {(delta || hint) && (
        <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1">
          {delta && deltaTone && (
            <span className={cn("inline-flex items-center gap-1 text-sm tabular-nums", DELTA_TONE_CLASS[deltaTone])}>
              {/* 方向图标为主要载体,颜色仅辅助(色非唯一) */}
              <Icon icon={delta.direction === "up" ? TrendingUp : TrendingDown} size="sm" />
              {delta.value}
            </span>
          )}
          {hint && <span className="text-xs text-ink-sub">{hint}</span>}
        </div>
      )}
    </div>
  );
}
