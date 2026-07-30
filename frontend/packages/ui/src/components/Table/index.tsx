/**
 * Table:列定义驱动的泛型数据表格(§5.1)。
 * 支持列对齐/等宽数字列/可排序表头(aria-sort)/行点击(键盘可达)/
 * 加载骨架与空态插槽;宽屏溢出时在容器内独立横向滚动,不带动整页。
 */
import type { KeyboardEvent, MouseEvent, ReactNode } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

export interface TableColumn<T> {
  /** 列唯一标识;无 render 时按此键从行对象取值,也是排序回调的 key */
  key: string;
  header: ReactNode;
  align?: "left" | "right" | "center";
  /** 列宽(如 "120px" / "20%"),经 style 传入(尺寸计算属例外,不违反零任意值) */
  width?: string;
  /** 数字/哈希列:font-mono tabular-nums,保证纵向对齐 */
  mono?: boolean;
  sortable?: boolean;
  render?: (row: T) => ReactNode;
}

export interface TableProps<T> {
  columns: TableColumn<T>[];
  data: T[];
  /** 行唯一键,用于 React key */
  rowKey: (row: T) => string;
  loading?: boolean;
  /** loading 时渲染的骨架行数,默认 5 */
  skeletonRows?: number;
  /** 空态插槽(data 为空且非 loading 时占满整行居中渲染),由页面注入引导内容 */
  empty?: ReactNode;
  /** 当前排序状态(受控),与 onSortChange 配合 */
  sort?: { key: string; direction: "asc" | "desc" };
  onSortChange?: (key: string) => void;
  /** 传入后整行可点、可聚焦、可回车触发 */
  onRowClick?: (row: T) => void;
  className?: string;
}

/** 对齐方式 → 文本对齐工具类 */
const ALIGN_CLASS = {
  left: "text-left",
  right: "text-right",
  center: "text-center",
} as const;

/**
 * 无 render 时的默认取值:按 key 从行对象读取。
 * null/undefined 渲染「—」占位;string/number/boolean 直接渲染;
 * 其他类型(对象/数组等)一律渲染「—」—— 复杂值必须经 column.render 显式渲染,
 * 防止对象直塞 React 导致运行时崩溃。
 */
function defaultCell<T>(row: T, key: string): ReactNode {
  const value = (row as Record<string, unknown>)[key];
  if (value == null) return "—";
  if (typeof value === "string" || typeof value === "number") return value;
  if (typeof value === "boolean") return String(value);
  return "—";
}

export function Table<T>({
  columns,
  data,
  rowKey,
  loading = false,
  skeletonRows = 5,
  empty,
  sort,
  onSortChange,
  onRowClick,
  className,
}: TableProps<T>) {
  // 交互元素守卫:事件源自行内按钮/链接/输入等交互子元素时不触发整行动作,
  // 防止单元格内操作冒泡误触 onRowClick、防止输入框空格被 preventDefault 吞掉
  const isFromInteractiveChild = (
    event: MouseEvent<HTMLTableRowElement> | KeyboardEvent<HTMLTableRowElement>,
  ) => {
    const target = event.target as HTMLElement;
    const hit = target.closest(
      'button, a, input, select, textarea, [role="button"], [role="menuitem"]',
    );
    return hit !== null && hit !== event.currentTarget;
  };

  // 行点击:守卫通过后触发整行动作
  const handleRowClick = (event: MouseEvent<HTMLTableRowElement>, row: T) => {
    if (isFromInteractiveChild(event)) return;
    onRowClick?.(row);
  };
  // 行键盘激活:onRowClick 存在时行可聚焦,Enter/Space 等同点击(仅守卫通过后才阻止默认滚动)
  const handleRowKeyDown = (event: KeyboardEvent<HTMLTableRowElement>, row: T) => {
    if (isFromInteractiveChild(event)) return;
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onRowClick?.(row);
    }
  };

  return (
    // 容器内独立横向滚动(overflow-x-auto):列过宽时只滚表格,不带动整页
    <div className={cn("overflow-x-auto rounded-lg border border-line bg-surface", className)}>
      <table className="w-full border-collapse">
        <thead>
          <tr className="bg-surface-sunken">
            {columns.map((column) => {
              const isSorted = sort?.key === column.key;
              return (
                <th
                  key={column.key}
                  scope="col"
                  // 可排序列向读屏暴露当前排序方向;未排序但可排序时为 none
                  aria-sort={
                    column.sortable
                      ? isSorted
                        ? sort?.direction === "asc"
                          ? "ascending"
                          : "descending"
                        : "none"
                      : undefined
                  }
                  style={column.width ? { width: column.width } : undefined}
                  className={cn(
                    "px-4 py-3 text-xs font-medium uppercase tracking-wider text-ink-sub",
                    ALIGN_CLASS[column.align ?? "left"],
                  )}
                >
                  {column.sortable ? (
                    <button
                      type="button"
                      onClick={() => onSortChange?.(column.key)}
                      className="inline-flex items-center gap-1 rounded-sm transition-colors duration-fast hover:text-ink focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2"
                    >
                      {column.header}
                      {/* 排序指示:未排序双向箭头,已排序按方向单箭头 */}
                      <Icon
                        icon={isSorted ? (sort?.direction === "asc" ? ArrowUp : ArrowDown) : ArrowUpDown}
                        size="sm"
                        className={cn(isSorted ? "text-primary" : "text-ink-faint")}
                      />
                    </button>
                  ) : (
                    column.header
                  )}
                </th>
              );
            })}
          </tr>
        </thead>
        <tbody>
          {loading ? (
            // 加载态:按 skeletonRows 渲染微光骨架行,预留空间防布局跳动
            Array.from({ length: skeletonRows }, (_, rowIndex) => (
              <tr key={`skeleton-${rowIndex}`} className="border-t border-line">
                {columns.map((column) => (
                  <td key={column.key} className="px-4 py-3">
                    <div className="h-4 w-full rounded-sm skeleton-shimmer" />
                  </td>
                ))}
              </tr>
            ))
          ) : data.length === 0 ? (
            // 空态:占满整行居中,内容由 empty 插槽注入(引导文案/行动按钮)
            <tr className="border-t border-line">
              <td colSpan={columns.length} className="px-4 py-12 text-center text-ink-sub">
                {empty}
              </td>
            </tr>
          ) : (
            data.map((row) => (
              // 可点行无法给出有意义的 aria-label(行内容任意):建议调用方额外提供
              // 显式操作列(如「查看」按钮)作为无障碍等价入口;此处保留 tabIndex 与 Enter/Space
              <tr
                key={rowKey(row)}
                onClick={onRowClick ? (event) => handleRowClick(event, row) : undefined}
                onKeyDown={onRowClick ? (event) => handleRowKeyDown(event, row) : undefined}
                tabIndex={onRowClick ? 0 : undefined}
                className={cn(
                  "border-t border-line transition-colors duration-fast hover:bg-surface-hover",
                  onRowClick &&
                    "cursor-pointer focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2",
                )}
              >
                {columns.map((column) => (
                  <td
                    key={column.key}
                    className={cn(
                      "px-4 py-3 text-base text-ink",
                      ALIGN_CLASS[column.align ?? "left"],
                      column.mono && "font-mono tabular-nums",
                    )}
                  >
                    {column.render ? column.render(row) : defaultCell(row, column.key)}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
