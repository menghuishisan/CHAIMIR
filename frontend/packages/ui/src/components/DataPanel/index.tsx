/**
 * DataPanel:一张白卡,内含工具条 + 数据区 + 分页页脚(规范 §6.5.2 / §5.1 布局组件)。
 *
 * 「筛什么 → 筛出来的东西 → 怎么翻页」本来就是一件事,收进同一张白卡:
 * 工具条与数据之间一条分隔线,页脚上边线收束(border 在这里是分隔线用途,不是圈盒子)。
 *
 * 三态(加载/空/错)由调用方放进 children —— 它们替换的是**数据区**,
 * 筛选与页脚在三态下的存留由页面决定(空态时通常仍要留筛选,否则用户没法改条件)。
 */
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface DataPanelProps {
  /**
   * 区域名(用户向),如「课程列表」。一页有多块 DataPanel 时必给 ——
   * 无名 section 不会作为地标暴露给读屏,用户没法在两块之间跳转。
   */
  label?: string;
  /** 工具条:传 `FilterBar`;渲染在卡内顶部,与数据区之间一条分隔线 */
  filter?: ReactNode;
  /** 数据区:表格 / 事件轴 / 空态 / 骨架 / 错误块 */
  children: ReactNode;
  /**
   * 页脚:列表放 `Pagination`(它自带「共 N 条」与页码,单页时只留总数);时间流放「加载更多」。
   * 单槽而非左右双槽:双槽会诱使调用方在左槽再写一遍记录总数,
   * 和 Pagination 自带的总数重复(§6.5.0 通则 1 不重复说同一件事)。
   */
  footer?: ReactNode;
  className?: string;
}

/** DataPanel 把工具条、数据区与页脚收进一张白卡;白卡不画边框,overflow-hidden 让内部圆角收得住。 */
export function DataPanel({ label, filter, children, footer, className }: DataPanelProps) {
  return (
    <section
      aria-label={label}
      className={cn("flex min-w-0 flex-col overflow-hidden rounded-lg bg-surface shadow-xs", className)}
    >
      {filter && <div className="border-b border-line px-4 py-3">{filter}</div>}
      {/* 数据区:min-w-0 防表格撑破容器产生非预期横向滚动(§6.4 红线) */}
      <div className="min-w-0 flex-1">{children}</div>
      {footer && <div className="border-t border-line px-4 py-3">{footer}</div>}
    </section>
  );
}
