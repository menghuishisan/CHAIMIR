/**
 * ChartContainer:图表容器与无障碍降级契约(规范 §8.2,强制)。
 * 统一提供:标题/描述头部、「图表 / 数据表」视图切换、加载(骨架)/错误(重试)/空态三态,
 * 以及必填的读屏摘要(ariaSummary)与数据表替代(dataTable)。所有图表必须包在本容器内。
 * 支持两种语境:paper(宣纸光面页面)与 dark(墨底沉浸舞台/深色面板),同一组件切换语义色,
 * 不在业务层复制第二个容器。
 */
import { useState, type ReactNode } from "react";
import { cn } from "../lib/cn";
import type { ChartContext } from "./palette";

/** 数据表替代:列头 + 行数据(与图表同源) */
export interface ChartDataTable {
  columns: string[];
  rows: Array<Array<string | number>>;
}

export interface ChartContainerProps {
  /** 图表标题 */
  title?: string;
  /** 用户向描述(说明图表回答什么问题) */
  description?: string;
  /** 内容区高度(px),默认 280;动态尺寸经 style 传入属例外 */
  height?: number;
  /** 加载中:渲染骨架微光块并预留高度 */
  loading?: boolean;
  /** 错误文案(用户向);存在即进入错误态 */
  error?: string;
  /** 错误态重试回调 */
  onRetry?: () => void;
  /** 数据为空 */
  isEmpty?: boolean;
  /** 空态引导文案 */
  emptyHint?: string;
  /** 必填:读屏用关键洞察摘要(如「近 7 日提交量整体上升,周五最高」) */
  ariaSummary: string;
  /** 必填:数据表替代,保证不依赖图形也能读取数据 */
  dataTable: ChartDataTable;
  /** 配色语境:paper=宣纸光面(默认),dark=墨底沉浸舞台/深色面板 */
  context?: ChartContext;
  /** 图表本体(如 TrendLineChart) */
  children: ReactNode;
  className?: string;
}

/** 视图切换小按钮的公共类(两态按钮,aria-pressed 表达当前视图);
 *  pressable 已含颜色过渡契约(theme.css),不再叠加 transition-colors */
const TOGGLE_BTN =
  "pressable rounded-sm px-2 py-1 text-xs focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2";

/** 两种语境的语义类:光面走 ink/surface/line,墨底走 on-dark/dark-* 系令牌 */
const STYLES: Record<
  ChartContext,
  {
    title: string;
    description: string;
    toggleOn: string;
    toggleOff: string;
    skeleton: string;
    errorBox: string;
    errorText: string;
    retryBtn: string;
    emptyBox: string;
    emptyText: string;
    tableWrap: string;
    tableHead: string;
    tableRow: string;
    tableCell: string;
  }
> = {
  paper: {
    title: "text-ink",
    description: "text-ink-sub",
    toggleOn: "bg-primary-soft text-primary",
    toggleOff: "text-ink-sub hover:bg-surface-hover hover:text-ink",
    skeleton: "skeleton-shimmer",
    errorBox: "border-danger-border bg-danger-bg",
    errorText: "text-danger",
    retryBtn: "border-line bg-surface text-ink hover:bg-surface-hover",
    emptyBox: "border-line bg-surface-sunken",
    emptyText: "text-ink-sub",
    tableWrap: "border-line",
    tableHead: "border-line bg-surface-sunken text-ink-sub",
    tableRow: "border-line",
    tableCell: "text-ink",
  },
  dark: {
    title: "text-on-dark",
    description: "text-on-dark-sub",
    toggleOn: "bg-dark-elevated text-accent",
    toggleOff: "text-on-dark-sub hover:bg-dark-elevated hover:text-on-dark",
    skeleton: "bg-dark-elevated",
    errorBox: "border-dark-line bg-dark-elevated",
    errorText: "text-on-dark-danger",
    retryBtn: "border-dark-line bg-dark-surface text-on-dark hover:bg-dark-elevated",
    emptyBox: "border-dark-line bg-dark-surface",
    emptyText: "text-on-dark-sub",
    tableWrap: "border-dark-line",
    tableHead: "border-dark-line bg-dark-elevated text-on-dark-sub",
    tableRow: "border-dark-line",
    tableCell: "text-on-dark",
  },
};

/**
 * ChartContainer:按 loading > error > isEmpty > 正常 的优先级渲染;
 * 正常态支持「图表 / 数据表」切换,容器整体以 figure + ariaSummary 暴露给读屏。
 */
export function ChartContainer({
  title,
  description,
  height = 280,
  loading = false,
  error,
  onRetry,
  isEmpty = false,
  emptyHint,
  ariaSummary,
  dataTable,
  context = "paper",
  children,
  className,
}: ChartContainerProps) {
  const [view, setView] = useState<"chart" | "table">("chart");
  // 三态互斥:仅正常态展示视图切换
  const isNormal = !loading && !error && !isEmpty;
  const styles = STYLES[context];

  return (
    <figure role="figure" aria-label={ariaSummary} className={cn("min-w-0", className)}>
      {(title || description || isNormal) && (
        <div className="mb-3 flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            {title && <div className={cn("text-md font-semibold", styles.title)}>{title}</div>}
            {description && <p className={cn("mt-0.5 text-sm", styles.description)}>{description}</p>}
          </div>
          {isNormal && (
            <div className="flex shrink-0 items-center gap-1" role="group" aria-label="图表视图切换">
              <button
                type="button"
                aria-pressed={view === "chart"}
                onClick={() => setView("chart")}
                className={cn(TOGGLE_BTN, view === "chart" ? styles.toggleOn : styles.toggleOff)}
              >
                图表
              </button>
              <button
                type="button"
                aria-pressed={view === "table"}
                onClick={() => setView("table")}
                className={cn(TOGGLE_BTN, view === "table" ? styles.toggleOn : styles.toggleOff)}
              >
                数据表
              </button>
            </div>
          )}
        </div>
      )}

      {/* 加载:骨架块预留同等高度,避免布局跳动(墨底语境用静态深色块,微光渐变为光面配方) */}
      {loading && (
        <div
          className={cn("rounded-md", styles.skeleton)}
          style={{ height }}
          role="status"
          aria-label="图表加载中"
        />
      )}

      {/* 错误:居中用户向文案 + 重试 */}
      {!loading && error && (
        <div
          style={{ height }}
          className={cn(
            "flex flex-col items-center justify-center gap-3 rounded-md border px-4 text-center",
            styles.errorBox,
          )}
          role="alert"
        >
          <p className={cn("text-sm", styles.errorText)}>{error}</p>
          {onRetry && (
            <button
              type="button"
              onClick={onRetry}
              // pressable 已含颜色过渡契约(theme.css),不再叠加 transition-colors
              className={cn(
                "pressable rounded-md border px-3 py-1.5 text-sm focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2",
                styles.retryBtn,
              )}
            >
              重试
            </button>
          )}
        </div>
      )}

      {/* 空态:引导而非空白坐标轴 */}
      {!loading && !error && isEmpty && (
        <div
          style={{ height }}
          className={cn(
            "flex flex-col items-center justify-center gap-1 rounded-md border px-4 text-center",
            styles.emptyBox,
          )}
        >
          <p className={cn("text-sm", styles.emptyText)}>暂无数据</p>
          {emptyHint && <p className={cn("text-xs", styles.emptyText)}>{emptyHint}</p>}
        </div>
      )}

      {/* 正常:图表或数据表(同一高度占位,切换不跳动) */}
      {isNormal && view === "chart" && (
        <div style={{ height }} className="min-w-0">
          {children}
        </div>
      )}
      {isNormal && view === "table" && (
        <div
          style={{ height }}
          className={cn("overflow-y-auto rounded-md border", styles.tableWrap)}
        >
          <table className="w-full border-collapse text-sm">
            <thead>
              <tr>
                {/* key 拼 index:列名可重复,不能单独作 key */}
                {dataTable.columns.map((col, colIndex) => (
                  <th
                    key={`${colIndex}-${col}`}
                    scope="col"
                    className={cn(
                      "sticky top-0 border-b px-3 py-2 text-left font-medium",
                      styles.tableHead,
                    )}
                  >
                    {col}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {dataTable.rows.map((row, rowIndex) => (
                <tr key={rowIndex} className={cn("border-b last:border-b-0", styles.tableRow)}>
                  {row.map((cell, cellIndex) => (
                    <td
                      key={cellIndex}
                      className={cn(
                        "px-3 py-2",
                        styles.tableCell,
                        typeof cell === "number" && "tabular-nums",
                      )}
                    >
                      {cell}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </figure>
  );
}
