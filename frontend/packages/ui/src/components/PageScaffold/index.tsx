/**
 * PageScaffold:日常页(光面语境)结构原语组合(规范 §6.5 资源页统一渲染范式)。
 * 提供 PageScaffold(容器)/ PageHeader(页面头)/ PageBody(主区 + 右侧动作区)/
 * PageSection(分组区)四个原语,四端资源页统一用它们组装,避免退化为裸堆叠。
 */
import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";
import { Icon } from "../../lib/icon";

/* ---------------------------------------------------------------- 容器 */

export interface PageScaffoldProps {
  children: ReactNode;
  className?: string;
}

/**
 * PageScaffold:页面内容容器 —— 居中、限宽、统一内边距。
 * 最大宽度引用布局令牌 --content-max(布局尺寸属内联 style 例外,SVG/类名无法表达该变量约束)。
 */
export function PageScaffold({ children, className }: PageScaffoldProps) {
  return (
    <div
      className={cn("mx-auto w-full px-8 py-7", className)}
      style={{ maxWidth: "var(--content-max)" }}
    >
      {children}
    </div>
  );
}

/* ---------------------------------------------------------------- 页面头 */

export interface PageHeaderProps {
  /** 面包屑插槽(kicker),渲染在标题上方 */
  kicker?: ReactNode;
  /** 页面主标题(h1,Display 字体) */
  title: string;
  /** 用户向描述文案 */
  description?: string;
  /** 页面图标(Lucide),渲染在标题左侧玉浅底容器内 */
  icon?: LucideIcon;
  /** 右侧页面级操作插槽 */
  actions?: ReactNode;
  className?: string;
}

/**
 * PageHeader:页面头 —— kicker + 图标 + h1 + 描述 + 右侧操作,底部细线收束。
 */
export function PageHeader({ kicker, title, description, icon, actions, className }: PageHeaderProps) {
  return (
    <header className={cn("mb-6 border-b border-line pb-5", className)}>
      {kicker && <div className="mb-3">{kicker}</div>}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex min-w-0 items-start gap-3">
          {icon && (
            <span className="mt-0.5 inline-flex shrink-0 rounded-lg bg-primary-soft p-2 text-primary">
              <Icon icon={icon} size="lg" />
            </span>
          )}
          <div className="min-w-0">
            <h1 className="font-display text-3xl text-ink">{title}</h1>
            {description && <p className="mt-1 text-sm text-ink-sub">{description}</p>}
          </div>
        </div>
        {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
      </div>
    </header>
  );
}

/* ---------------------------------------------------------------- 主体 */

export interface PageBodyProps {
  /** 右侧动作区(rail):桌面固定 320px 侧栏,窄屏移到主区之后单列 */
  rail?: ReactNode;
  children: ReactNode;
  className?: string;
}

/**
 * PageBody:主工作区布局 —— 有 rail 时桌面「主区 1fr + 右栏 320px」双栏,
 * max-lg 单列(DOM 顺序主区在前,rail 自然落在其后);无 rail 时单列直排。
 * 双栏用 flex + w-80(标准刻度,恰为 320px)实现,等价 grid 1fr/320px 且不引入任意值语法。
 */
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

/* ---------------------------------------------------------------- 分组区 */

export interface PageSectionProps {
  /** 分组标题 */
  title?: string;
  /** 分组说明(用户向) */
  description?: string;
  /** 分组右侧动作(如刷新/筛选) */
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}

/**
 * PageSection:页面内分组 —— 分组标题 + 说明 + 右侧动作 + 内容,组间以 mb-8 分隔。
 */
export function PageSection({ title, description, actions, children, className }: PageSectionProps) {
  const hasHeader = Boolean(title || description || actions);
  return (
    <section className={cn("mb-8", className)}>
      {hasHeader && (
        <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
          <div className="min-w-0">
            {title && <h2 className="text-lg font-semibold text-ink">{title}</h2>}
            {description && <p className="mt-0.5 text-sm text-ink-sub">{description}</p>}
          </div>
          {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
        </div>
      )}
      {children}
    </section>
  );
}
