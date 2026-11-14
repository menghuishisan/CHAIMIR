/**
 * PageScaffold:日常页结构原语(规范 §5.1 / §6.5)。
 * 提供 PageScaffold(容器)/ PageHeader(页头)/ PageBody(主区 + 右侧栏)/ PageSection(分组)四个原语;
 * 它们只管结构与间距,每一页的布局由该页的任务决定(§6.5.0),不套统一模子。
 * 当前位置在顶栏(§6.2),页头不再放面包屑。
 */
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

/* ---------------------------------------------------------------- 容器 */

export interface PageScaffoldProps {
  children: ReactNode;
  className?: string;
}

/**
 * PageScaffold:页面内容容器 —— 按纸宽铺满 + 分档内距(§6.4.1 规则 1)。
 * <md 16;md 24;≥lg 左右 36、上 28、下 40。
 * 不做全局 max-width 居中:限宽会在宽屏把内容缩成居中一根柱子;需要限宽的内容由各页自己写。
 */
export function PageScaffold({ children, className }: PageScaffoldProps) {
  return (
    <div className={cn("min-h-full w-full min-w-0 p-4 pb-10 md:p-6 md:pb-10 lg:px-9 lg:pb-10 lg:pt-7", className)}>
      {children}
    </div>
  );
}

/* ---------------------------------------------------------------- 页头 */

/** 页头属性行的一项:只给 value 时只显示值(如「2026 秋季」),值本身说不清时加 label(如「镜像版本 3 个」) */
export interface PageHeaderMetaItem {
  label?: string;
  value: ReactNode;
}

export interface PageHeaderProps {
  /** 页面标题(h1);详情页就是对象名 */
  title: string;
  /** 状态徽记,紧跟标题;状态必须带文字,不靠颜色单一传达(FE-2) */
  status?: ReactNode;
  /** 一行关键属性;值为空的项自动略过 */
  meta?: PageHeaderMetaItem[];
  /** 一句说明;只在新用户确实需要的页面保留 */
  description?: string;
  /** 右侧主操作(每页最多一个实底主按钮,§6.5.0 通则 3);窄屏占满一行 */
  actions?: ReactNode;
  className?: string;
}

/**
 * PageHeader:「标题 + 状态徽记 + 一行属性或说明 + 右侧主操作」,不带面包屑、图标和底线。
 * 一页只有一个 h1。
 */
export function PageHeader({ title, status, meta, description, actions, className }: PageHeaderProps) {
  const metaItems = (meta ?? []).filter(
    (item) => item.value !== null && item.value !== undefined && item.value !== false && item.value !== "",
  );
  return (
    <header className={cn("mb-6 flex flex-wrap items-start justify-between gap-x-6 gap-y-4", className)}>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <h1 className="min-w-0 break-words text-2xl font-bold leading-tight text-ink md:text-3xl">{title}</h1>
          {status}
        </div>
        {metaItems.length > 0 && (
          <p className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm text-ink-sub">
            {metaItems.map((item, index) => (
              <span key={item.label ?? index} className="inline-flex items-center gap-2">
                {index > 0 && (
                  <span aria-hidden="true" className="text-ink-faint">
                    ·
                  </span>
                )}
                {item.label ? (
                  <span>
                    {item.label} <span className="text-ink">{item.value}</span>
                  </span>
                ) : (
                  item.value
                )}
              </span>
            ))}
          </p>
        )}
        {description && <p className="mt-2 max-w-3xl text-sm text-ink-sub">{description}</p>}
      </div>
      {actions && (
        <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto sm:justify-end">{actions}</div>
      )}
    </header>
  );
}

/* ---------------------------------------------------------------- 主区 + 右侧栏 */

export interface PageBodyProps {
  /** 右侧栏:桌面固定 320px,窄屏移到主区之后单列 */
  rail?: ReactNode;
  children: ReactNode;
  className?: string;
}

/** PageBody:主区 + 可选右侧栏;<lg 右侧栏下沉到主区之后 */
export function PageBody({ rail, children, className }: PageBodyProps) {
  if (!rail) {
    return <div className={cn("min-w-0", className)}>{children}</div>;
  }
  return (
    <div className={cn("flex flex-col gap-6 lg:flex-row", className)}>
      <div className="min-w-0 flex-1">{children}</div>
      <aside className="min-w-0 lg:w-80 lg:shrink-0">{rail}</aside>
    </div>
  );
}

/* ---------------------------------------------------------------- 分组 */

export interface PageSectionProps {
  /** 分组标题 */
  title?: string;
  /** 分组说明(用户向) */
  description?: string;
  /** 分组右侧动作(刷新、导出、新建、视图切换) */
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}

/**
 * PageSection:页面内分组 —— 分组标题 + 说明 + 右侧动作 + 内容,组间以 mb-8 分隔;永远不画容器(§6.5.1)。
 */
export function PageSection({ title, description, actions, children, className }: PageSectionProps) {
  const hasHeader = Boolean(title || description || actions);
  return (
    <section className={cn("mb-8", className)}>
      {hasHeader && (
        <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            {title && <h2 className="text-lg font-bold text-ink">{title}</h2>}
            {description && <p className="mt-0.5 text-sm text-ink-sub">{description}</p>}
          </div>
          {actions && (
            <div className="flex max-w-full flex-wrap items-center gap-2 sm:justify-end">
              {actions}
            </div>
          )}
        </div>
      )}
      {children}
    </section>
  );
}
