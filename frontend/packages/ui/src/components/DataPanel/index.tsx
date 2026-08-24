/**
 * DataPanel:一块抬起片,内含筛选井 + 数据区 + 分页页脚
 * (规范 §6.5.3 第 ①「资源列表」与第 ⑥「时间流」族)。
 *
 * 存在的理由是 §6.5.1 红线第 3 条:井只能出现在抬起片内部。
 * 此前筛选井直接摆在光面上、数据表另成一块抬起片,一个逻辑数据区被渲染成两个互不相连的盒子,
 * 分页又是第三个 —— 三个矩形并排堆在光面上,这正是「套壳感」的来源之一。
 * DataPanel 把「筛什么 → 筛出来的东西 → 怎么翻页」收进同一张纸片:
 * 它们本来就是一件事,视觉上也该是一块。
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
  /**
   * 筛选区:传 `FilterBar`。渲染在片内顶部、四周留内距 ——
   * 井要看起来是「这块纸上被压下去的一条」,所以不能贴片边。
   */
  filter?: ReactNode;
  /** 数据区:表格 / 事件轴 / 空态 / 骨架 / 错误块 */
  children: ReactNode;
  /**
   * 页脚:资源列表族放 `Pagination`(它自带「共 N 条」与页码,单页时只留总数);
   * 时间流族放「加载更多」。
   * 单槽而非左右双槽:双槽会诱使调用方在左槽再写一遍记录总数,
   * 和 Pagination 自带的总数重复(§6.5.0 通则 1 不重复说同一件事)。
   */
  footer?: ReactNode;
  className?: string;
}

export function DataPanel({ label, filter, children, footer, className }: DataPanelProps) {
  return (
    // 抬起片(§6.5.1 第 1 级):底色 + 落影,不画边框;overflow-hidden 让内部圆角收得住
    <section
      aria-label={label}
      className={cn(
        "flex min-w-0 flex-col overflow-hidden rounded-lg bg-surface shadow-xs",
        className,
      )}
    >
      {filter && <div className="p-3 pb-0 md:p-4 md:pb-0">{filter}</div>}
      {/* 数据区:min-w-0 防表格撑破容器产生非预期横向滚动(§6.4 红线) */}
      <div className="min-w-0 flex-1">{children}</div>
      {footer && (
        // 页脚用上边线与数据区分隔 —— border 在这里是分隔线用途,不是圈盒子(§6.5.1)
        <div className="border-t border-line px-4 py-3">{footer}</div>
      )}
    </section>
  );
}
