/**
 * Pagination:分页控件(§5.1 / §6.5 数据区)。
 * 上一页/下一页 + 窗口化页码(首末页常显、中间省略),当前页玉实底;
 * 右侧展示总条数。total<=pageSize(单页)时仍渲染但翻页禁用——
 * 是否在单页时整体隐藏由页面自行决定,组件不做隐藏。
 */
import { ChevronLeft, ChevronRight } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export interface PaginationProps {
  /** 当前页(从 1 起) */
  page: number;
  pageSize: number;
  /** 总记录数(非总页数) */
  total: number;
  onPageChange: (page: number) => void;
  className?: string;
}

/** 省略号占位:左右两侧最多各一个,用不同标记保证 React key 稳定 */
type PageItem = number | "gap-left" | "gap-right";

/**
 * 计算窗口化页码:首页/末页常显,当前页左右各留一位,
 * 与首末之间出现断档时以省略号占位;总页数少时全部展示。
 */
function buildPageItems(page: number, totalPages: number): PageItem[] {
  // 页数不多时无需省略,直接全量
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, index) => index + 1);
  }
  const items: PageItem[] = [1];
  const windowStart = Math.max(2, page - 1);
  const windowEnd = Math.min(totalPages - 1, page + 1);
  if (windowStart > 2) items.push("gap-left");
  for (let current = windowStart; current <= windowEnd; current += 1) {
    items.push(current);
  }
  if (windowEnd < totalPages - 1) items.push("gap-right");
  items.push(totalPages);
  return items;
}

/** 页码/翻页按钮共用的基础类:固定高度、可按压、焦点环、禁用态、命中区扩到 44×44(hit-target) */
const BUTTON_BASE =
  "hit-target inline-flex h-8 min-w-8 items-center justify-center rounded-md px-1 text-sm tabular-nums pressable focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 disabled:opacity-50 disabled:pointer-events-none";

export function Pagination({ page, pageSize, total, onPageChange, className }: PaginationProps) {
  // pageSize 防御:<=0 或小数会算出 Infinity/NaN 页,先夹到至少 1 的整数
  const safePageSize = Math.max(1, Math.floor(pageSize));
  // 至少 1 页:total 为 0 时也渲染禁用态骨干,避免布局跳动
  const totalPages = Math.max(1, Math.ceil(total / safePageSize));
  const items = buildPageItems(page, totalPages);

  return (
    <nav aria-label="分页" className={cn("flex flex-wrap items-center gap-1", className)}>
      <button
        type="button"
        aria-label="上一页"
        disabled={page <= 1}
        onClick={() => onPageChange(page - 1)}
        className={cn(BUTTON_BASE, "text-ink-sub hover:bg-surface-hover hover:text-ink")}
      >
        <Icon icon={ChevronLeft} size="sm" />
      </button>
      {items.map((item) =>
        typeof item === "number" ? (
          <button
            key={item}
            type="button"
            aria-current={item === page ? "page" : undefined}
            onClick={() => onPageChange(item)}
            className={cn(
              BUTTON_BASE,
              item === page
                ? "bg-primary text-on-solid"
                : "text-ink-sub hover:bg-surface-hover hover:text-ink",
            )}
          >
            {item}
          </button>
        ) : (
          // 省略号仅视觉提示,读屏跳过
          <span key={item} aria-hidden="true" className="px-1 text-sm text-ink-faint">
            …
          </span>
        ),
      )}
      <button
        type="button"
        aria-label="下一页"
        disabled={page >= totalPages}
        onClick={() => onPageChange(page + 1)}
        className={cn(BUTTON_BASE, "text-ink-sub hover:bg-surface-hover hover:text-ink")}
      >
        <Icon icon={ChevronRight} size="sm" />
      </button>
      <span className="ml-auto pl-3 text-sm text-ink-sub tabular-nums">共 {total} 条</span>
    </nav>
  );
}
