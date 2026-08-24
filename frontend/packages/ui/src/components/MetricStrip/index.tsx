/**
 * MetricStrip:内联指标摘要(规范 §6.5.3 第 ① 族「资源列表」)。
 *
 * 为什么不用 Stat 卡:资源列表页的主体是列表,读者来这页是为了找一条记录。
 * 四张 `p-5` 的 Stat 大卡带 Display 字号数字要占 155px,把表格第一行推到折叠线以下,
 * 视觉权重也压过了真正的主角。指标在这一族的角色是「总量参照」而不是页面主体,
 * 所以降为一行内联摘要:数值仍是全量口径(§6.5.4),但字号退一档、不再各自成盒。
 *
 * 需要大卡的是「看板族」(§6.5.3 第 ② 族)——那里数字确实是主体,继续用 Stat。
 *
 * 响应式(§6.4.1 规则 2):`<md` 压成 2×2,一行一项、标签与数值同行;禁止竖排大卡。
 */
import type { ReactNode } from "react";
import { cn } from "../../lib/cn";

export interface MetricStripItem {
  /** 指标名(用户向) */
  label: string;
  /** 数值;拿不到全量聚合就不要这一项(§6.5.4),不做近似 */
  value: ReactNode;
  /** 口径补充,如「不受下方筛选影响」;只在会引起误读时给 */
  hint?: string;
}

export interface MetricStripProps {
  /** 无障碍名称,如「课程总量摘要」 */
  label: string;
  items: MetricStripItem[];
  className?: string;
}

export function MetricStrip({ label, items, className }: MetricStripProps) {
  return (
    <dl
      aria-label={label}
      className={cn(
        // <md:2 列网格,每项内部横排(标签左、数值右),行间用分隔线而不是盒子
        "grid grid-cols-2 gap-x-4 gap-y-2",
        // ≥md:一行排开,项与项之间用竖分隔线;md:flex 让项宽随内容而不是等分
        "md:flex md:flex-wrap md:items-stretch md:gap-x-7 md:gap-y-3",
        className,
      )}
    >
      {items.map((item, index) => (
        <div
          key={item.label}
          className={cn(
            // <md:每项一行,标签与数值同行基线对齐,底部一条分隔线代替卡片边界
            "flex items-baseline justify-between gap-3 border-b border-line pb-1.5",
            // ≥md:标签在上数值在下,竖线分隔;首项不画竖线
            "md:flex-col md:items-start md:justify-start md:gap-0 md:border-b-0 md:pb-0",
            index > 0 && "md:border-l md:border-line md:pl-7",
          )}
        >
          <dt className="text-xs text-ink-sub">{item.label}</dt>
          <dd className="text-lg font-semibold text-ink tabular-nums md:mt-0.5 md:text-xl">
            {item.value}
          </dd>
          {/* 口径补充只在 ≥md 出现:窄屏一行放不下三段信息,口径让位于数值本身 */}
          {item.hint && <p className="hidden text-xs text-ink-sub md:block">{item.hint}</p>}
        </div>
      ))}
    </dl>
  );
}
