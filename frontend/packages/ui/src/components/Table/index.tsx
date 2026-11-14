/**
 * Table:列定义驱动的泛型数据表格(§5.1)。
 * 支持列对齐/等宽数字列/可排序表头(aria-sort)/加载骨架与空态插槽;
 * 宽屏溢出时在容器内独立横向滚动,不带动整页。
 *
 * 行的两种可点方式(二选一):
 * - `getRowHref` + `onNavigate`:整行是链接(规范 §6.5.0 通则 4)。主列(primary,缺省第一列)的内容包在真实 `<a>` 里,
 *   键盘焦点、读屏、右键复制链接都落在它上;行内其它位置的点击按 linkActivation 处理 ——
 *   普通左键交给路由做本页跳转,中键与 Ctrl/⌘/Shift 点击在新标签页打开;悬停有底色,行尾箭头淡入。
 *   拖动选择文字时不跳转,行内按钮、勾选框等交互元素不触发整行。
 * - `onRowClick`:整行触发一个页内动作(如打开详情抽屉),行可聚焦、回车与空格触发。
 *
 * 传 `mobileCard` 时 `<md` 改渲行卡(§6.4.1 规则 3),两种形态同源一份 data 与一份列定义。
 */
import type { KeyboardEvent, MouseEvent, ReactNode } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown, ChevronRight } from "lucide-react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";
import { linkActivation } from "../../lib/linkActivation";

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
  /**
   * 整行是链接时,链接包在这一列(通常是对象名所在列);都不标时用第一列。
   * 这一列的内容不得再放按钮或链接(链接里不能套交互元素)。
   */
  primary?: boolean;
  render?: (row: T) => ReactNode;
}

/** 窄屏行卡的内容(§6.4.1 规则 3):主字段一行、次要信息一行、状态徽标与箭头在右 */
export interface TableMobileCard {
  /** 主字段,如课程名;整行是链接时它就是链接文字 */
  title: ReactNode;
  /** 次要信息一行,如「3 学分 · 2026/09/01 起 · CHAIN26A」 */
  meta?: ReactNode;
  /** 状态徽标槽位 */
  badge?: ReactNode;
  /** 行尾动作槽位;不传且行可点时显示右向箭头示意可进入 */
  action?: ReactNode;
}

interface TableBaseProps<T> {
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
  /**
   * `<md` 的行卡渲染(§6.4.1 规则 3)。传入后窄屏改渲染行卡、不再横滚表格 ——
   * 把六列挤进 343px 或让用户横滚去找「状态」列,两者都不算可用。
   * 列定义与行卡同源于一份 data,不产生第二套列表实现。
   */
  mobileCard?: (row: T) => TableMobileCard;
  /**
   * 是否自带白卡外观(底色 + 落影 + 圆角),默认 true。
   * 放进 `DataPanel` 时传 false —— 那时容器本身就是白卡,表格再画一层就成了卡里套卡。
   */
  elevated?: boolean;
  className?: string;
}

/** 行的可点方式:整行链接(须同时给 onNavigate,交给路由做本页跳转)与页内动作二选一 */
type TableRowAction<T> =
  | { getRowHref: (row: T) => string; onNavigate: (href: string) => void; onRowClick?: never }
  | { getRowHref?: never; onNavigate?: never; onRowClick?: (row: T) => void };

export type TableProps<T> = TableBaseProps<T> & TableRowAction<T>;

/** 对齐方式 → 文本对齐工具类 */
const ALIGN_CLASS = {
  left: "text-left",
  right: "text-right",
  center: "text-center",
} as const;

/** 行内交互元素:事件源自它们时整行不响应 */
const INTERACTIVE_SELECTOR =
  'button, a, input, select, textarea, label, [role="button"], [role="menuitem"], [role="checkbox"], [role="switch"]';

/** 链接焦点环(键盘可见,鼠标点击不显示) */
const LINK_FOCUS_CLASS =
  "rounded-sm outline-hidden focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent";

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

/** isFromInteractiveChild 判断事件是否来自行内的按钮、链接、输入等交互子元素(行元素自身不算) */
function isFromInteractiveChild(event: MouseEvent<HTMLElement> | KeyboardEvent<HTMLElement>): boolean {
  const hit = (event.target as HTMLElement).closest(INTERACTIVE_SELECTOR);
  return hit !== null && hit !== event.currentTarget;
}

/** hasTextSelection 判断用户是否刚在拖动选择文字:选文字时松开鼠标不应触发整行跳转 */
function hasTextSelection(): boolean {
  const selection = window.getSelection();
  return Boolean(selection && !selection.isCollapsed && selection.toString().trim() !== "");
}

/** openInNewTab 在新标签页打开站内地址;noopener 防止新页面反向操纵本页 */
function openInNewTab(href: string): void {
  window.open(href, "_blank", "noopener,noreferrer");
}

/** Table 列定义驱动的泛型数据表格;窄屏可换成行卡 */
export function Table<T>({
  columns,
  data,
  rowKey,
  loading = false,
  skeletonRows = 5,
  empty,
  sort,
  onSortChange,
  getRowHref,
  onNavigate,
  onRowClick,
  mobileCard,
  elevated = true,
  className,
}: TableProps<T>) {
  // 整行链接模式:类型上 getRowHref 与 onNavigate 成对出现,这里合成一个对象便于收窄
  const link = getRowHref && onNavigate ? { hrefOf: getRowHref, navigate: onNavigate } : null;
  const primaryKey = (columns.find((column) => column.primary) ?? columns[0])?.key;
  // 整行链接时行尾多一列装饰箭头,空态与骨架的跨列数要算上它
  const columnCount = columns.length + (link ? 1 : 0);

  /** 链接上的点击:普通左键交给路由;修饰键与中键保留浏览器默认行为(新标签页打开) */
  const handleAnchorClick = (event: MouseEvent<HTMLAnchorElement>, href: string) => {
    if (!link || event.defaultPrevented) return;
    if (linkActivation(event) !== "same-tab") return;
    event.preventDefault();
    link.navigate(href);
  };

  /** 行内链接以外位置的点击:按点击方式本页跳转或新标签页打开 */
  const handleLinkRowClick = (event: MouseEvent<HTMLElement>, href: string) => {
    if (!link || isFromInteractiveChild(event) || hasTextSelection()) return;
    const activation = linkActivation(event);
    if (activation === "same-tab") link.navigate(href);
    else if (activation === "new-tab") openInNewTab(href);
  };

  /** 中键点击行(不在链接上):新标签页打开;链接上的中键由浏览器自己处理,不重复打开 */
  const handleLinkRowAuxClick = (event: MouseEvent<HTMLElement>, href: string) => {
    if (event.button !== 1 || isFromInteractiveChild(event)) return;
    openInNewTab(href);
  };

  /** 中键按下时阻止浏览器进入自动滚动,否则松开后不会触发 auxclick */
  const handleLinkRowMouseDown = (event: MouseEvent<HTMLElement>) => {
    if (event.button === 1 && !isFromInteractiveChild(event)) event.preventDefault();
  };

  // 页内动作模式:守卫通过后触发整行动作
  const handleRowClick = (event: MouseEvent<HTMLElement>, row: T) => {
    if (isFromInteractiveChild(event)) return;
    onRowClick?.(row);
  };
  // 行键盘激活:Enter/Space 等同点击(仅守卫通过后才阻止默认滚动,不影响行内输入框)
  const handleRowKeyDown = (event: KeyboardEvent<HTMLElement>, row: T) => {
    if (isFromInteractiveChild(event)) return;
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onRowClick?.(row);
    }
  };

  /** renderCell 渲染一个单元格内容;整行链接时主列包进真实链接 */
  const renderCell = (row: T, column: TableColumn<T>): ReactNode => {
    const content = column.render ? column.render(row) : defaultCell(row, column.key);
    if (!link || column.key !== primaryKey) return content;
    const href = link.hrefOf(row);
    return (
      <a href={href} onClick={(event) => handleAnchorClick(event, href)} className={cn("block min-w-0", LINK_FOCUS_CLASS)}>
        {content}
      </a>
    );
  };

  /** renderMobileCard 渲染一张窄屏行卡 */
  const renderMobileCard = (row: T, card: TableMobileCard): ReactNode => {
    if (link) {
      // 整卡链接:标题链接的 ::before 铺满整张卡,行尾动作浮在其上单独可点;焦点环画在 ::before 上
      const href = link.hrefOf(row);
      return (
        <div className="relative flex items-center gap-3 px-4 py-3 transition-colors duration-fast hover:bg-surface-hover">
          <div className="min-w-0 flex-1">
            <a
              href={href}
              onClick={(event) => handleAnchorClick(event, href)}
              className="block truncate text-base font-medium text-ink outline-hidden before:absolute before:inset-0 focus-visible:before:outline-2 focus-visible:before:-outline-offset-2 focus-visible:before:outline-accent"
            >
              {card.title}
            </a>
            {card.meta && <div className="mt-0.5 truncate text-sm text-ink-sub">{card.meta}</div>}
          </div>
          {card.badge && <div className="shrink-0">{card.badge}</div>}
          {card.action ? (
            <div className="relative z-base shrink-0">{card.action}</div>
          ) : (
            <Icon icon={ChevronRight} size="sm" className="shrink-0 text-ink-faint" />
          )}
        </div>
      );
    }
    const cardLabel =
      typeof card.title === "string" || typeof card.title === "number" ? String(card.title) : undefined;
    return (
      // 页内动作的整卡:role=button + tabIndex,回车与空格触发;行尾动作由守卫排除
      <div
        role={onRowClick ? "button" : undefined}
        tabIndex={onRowClick ? 0 : undefined}
        onClick={onRowClick ? (event) => handleRowClick(event, row) : undefined}
        onKeyDown={onRowClick ? (event) => handleRowKeyDown(event, row) : undefined}
        aria-label={onRowClick ? cardLabel : undefined}
        className={cn(
          "flex items-center gap-3 px-4 py-3",
          onRowClick &&
            "cursor-pointer transition-colors duration-fast hover:bg-surface-hover focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2",
        )}
      >
        <div className="min-w-0 flex-1">
          <div className="truncate text-base text-ink">{card.title}</div>
          {card.meta && <div className="mt-0.5 truncate text-sm text-ink-sub">{card.meta}</div>}
        </div>
        {card.badge && <div className="shrink-0">{card.badge}</div>}
        {card.action ??
          (onRowClick ? <Icon icon={ChevronRight} size="sm" className="shrink-0 text-ink-faint" /> : null)}
      </div>
    );
  };

  return (
    // 白卡(规范 §6.5.1):不画外框,层次靠底色与落影;elevated=false 时交给外层容器。
    // 容器内独立横向滚动(overflow-x-auto):列过宽时只滚表格,不带动整页;
    // 给了 mobileCard 的表在 <md 改渲行卡,那时不需要横滚。
    <div
      className={cn(
        "min-w-0",
        elevated && "rounded-lg bg-surface shadow-xs",
        mobileCard ? "md:overflow-x-auto" : "overflow-x-auto",
        className,
      )}
      aria-busy={loading || undefined}
    >
      {/* 窄屏行卡(§6.4.1 规则 3):与表格同源一份 data,列定义不复制第二遍 */}
      {mobileCard && (
        <ul className="md:hidden">
          {loading
            ? Array.from({ length: skeletonRows }, (_, index) => (
                <li key={`card-skeleton-${index}`} className="border-b border-line px-4 py-3 last:border-b-0">
                  <div className="h-4 w-2/3 rounded-sm skeleton-shimmer" />
                  <div className="mt-2 h-3 w-1/2 rounded-sm skeleton-shimmer" />
                </li>
              ))
            : data.length === 0
              ? <li className="px-4 py-12 text-center text-ink-sub">{empty}</li>
              : data.map((row) => (
                  <li key={rowKey(row)} className="border-b border-line last:border-b-0">
                    {renderMobileCard(row, mobileCard(row))}
                  </li>
                ))}
        </ul>
      )}

      <table className={cn("w-full border-collapse", mobileCard && "hidden md:table")}>
        <thead>
          <tr className="border-b border-line">
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
                  className={cn("px-4 py-3 text-sm font-medium text-ink-sub", ALIGN_CLASS[column.align ?? "left"])}
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
            {/* 行尾箭头列是纯装饰,表头与各行都对读屏隐藏,列数保持一致 */}
            {link && <td aria-hidden="true" className="w-10" />}
          </tr>
        </thead>
        <tbody>
          {loading ? (
            // 加载态:按 skeletonRows 渲染微光骨架行,预留空间防布局跳动
            Array.from({ length: skeletonRows }, (_, rowIndex) => (
              <tr key={`skeleton-${rowIndex}`} className="border-t border-line">
                {columns.map((column) => (
                  <td key={column.key} className="px-4 py-3.5">
                    <div className="h-4 w-full rounded-sm skeleton-shimmer" />
                  </td>
                ))}
                {link && <td aria-hidden="true" />}
              </tr>
            ))
          ) : data.length === 0 ? (
            // 空态:占满整行居中,内容由 empty 插槽注入(引导文案/行动按钮)
            <tr className="border-t border-line">
              <td colSpan={columnCount} className="px-4 py-12 text-center text-ink-sub">
                {empty}
              </td>
            </tr>
          ) : (
            data.map((row) => {
              const href = link ? link.hrefOf(row) : undefined;
              return (
                <tr
                  key={rowKey(row)}
                  onClick={
                    href !== undefined
                      ? (event) => handleLinkRowClick(event, href)
                      : onRowClick
                        ? (event) => handleRowClick(event, row)
                        : undefined
                  }
                  onAuxClick={href !== undefined ? (event) => handleLinkRowAuxClick(event, href) : undefined}
                  onMouseDown={href !== undefined ? handleLinkRowMouseDown : undefined}
                  onKeyDown={onRowClick ? (event) => handleRowKeyDown(event, row) : undefined}
                  // 整行链接时焦点落在主列的链接上;页内动作时行本身可聚焦
                  tabIndex={onRowClick ? 0 : undefined}
                  className={cn(
                    "group border-t border-line transition-colors duration-fast hover:bg-surface-hover",
                    (href !== undefined || onRowClick) && "cursor-pointer",
                    onRowClick &&
                      "focus-visible:outline-2 focus-visible:outline-accent focus-visible:-outline-offset-2",
                  )}
                >
                  {columns.map((column) => (
                    <td
                      key={column.key}
                      className={cn(
                        "px-4 py-3.5 text-base text-ink",
                        ALIGN_CLASS[column.align ?? "left"],
                        column.mono && "font-mono tabular-nums",
                      )}
                    >
                      {renderCell(row, column)}
                    </td>
                  ))}
                  {href !== undefined && (
                    <td aria-hidden="true" className="w-10 pr-3 text-right">
                      {/* 悬停或行内获得焦点时淡入;触屏没有悬停,直接显示 */}
                      <Icon
                        icon={ChevronRight}
                        size="sm"
                        className="inline-block text-ink-faint opacity-0 transition-opacity duration-fast group-hover:opacity-100 group-focus-within:opacity-100 pointer-coarse:opacity-100"
                      />
                    </td>
                  )}
                </tr>
              );
            })
          )}
        </tbody>
      </table>
    </div>
  );
}
