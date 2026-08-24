/**
 * ShareDonutChart:占比环图(规范 §8.1「部分对整体,≤5 类」)。
 *
 * 只回答「各部分占整体多少」。判据有两条硬约束,不满足就不该用这个图形:
 * ① 类别数 2–5。多于 5 类时环上每片都太薄,读者只能靠图例读数字,那就直接用 CompareBarChart;
 * ② 各部分之和构成整体。加不成整体的数据(如「进行中 / 已结课」两个互不排斥的口径)不是占比。
 * 违反时**抛错而不静默降级** —— 与 palette.ts 读令牌失败的处理一致(CLAUDE.md §8 错误不静默失败):
 * 静默把第 6 片并进「其他」会悄悄改变数据含义,静默改画柱状会让同一份代码在不同数据下长出两种图。
 * 调用方拿到的是明确的开发期错误,应当在业务侧显式聚合出「其他」这一类,或换成柱状。
 *
 * 中心留给总量:环形的空心本来就是留白,把总数放进去等于免费多一个读数位。
 * 图例同时给名称、数值与百分比 —— 颜色不是唯一区分手段(§8.2)。
 * 图例**不做点击切换显隐**:环表达的是「各部分构成整体」,隐掉一片后剩下的百分比就不再对整体成立,
 * 那是把图形本身的语义弄坏。§8.2 的「图例可点切换」针对多系列折线/柱状,不适用于占比。
 * 应包在 ChartContainer 内使用。
 */
import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";
import { useReducedMotion } from "../hooks/useReducedMotion";
import { cn } from "../lib/cn";
import { useChartColors, type ChartContext } from "./palette";

/** 一个占比分片 */
export interface ShareSlice {
  /** 唯一标识,用作 React key 与 Recharts 的 nameKey */
  key: string;
  /** 用户向名称,如「学生」 */
  name: string;
  value: number;
}

export interface ShareDonutChartProps {
  /** 分片,2–5 个;顺序即绘制顺序(建议按数值降序,读者从大到小扫) */
  slices: ShareSlice[];
  /** 中心总量下方的说明,如「在册账号」 */
  totalLabel: string;
  /** 图表高度(px),默认 240 */
  height?: number;
  /** 配色语境:paper=宣纸光面(默认),dark=墨底沉浸舞台/深色面板 */
  context?: ChartContext;
  className?: string;
}

/** 环图的类别数硬边界(规范 §8.1) */
const MIN_SLICES = 2;
const MAX_SLICES = 5;

/** 两种语境的语义类 */
const STYLES: Record<ChartContext, { total: string; label: string; name: string }> = {
  paper: { total: "text-ink", label: "text-ink-sub", name: "text-ink" },
  dark: { total: "text-on-dark", label: "text-on-dark-sub", name: "text-on-dark" },
};

export function ShareDonutChart({
  slices,
  totalLabel,
  height = 240,
  context = "paper",
  className,
}: ShareDonutChartProps) {
  // 契约校验放在一切之前:选型错了就没有「先渲染一半」的意义,也不该先去读令牌
  if (slices.length < MIN_SLICES || slices.length > MAX_SLICES) {
    throw new Error(
      `ShareDonutChart 需要 ${MIN_SLICES}–${MAX_SLICES} 个分片,收到 ${slices.length} 个:` +
        `占比环超过 5 类时每片过薄不可读(规范 §8.1)。请在业务侧显式聚合出「其他」一类,或改用 CompareBarChart。`,
    );
  }

  const colors = useChartColors(context);
  const reducedMotion = useReducedMotion();
  const styles = STYLES[context];

  const total = slices.reduce((sum, slice) => sum + slice.value, 0);
  /** 百分比:总量为 0 时按 0% 呈现,不产生 NaN */
  const percentOf = (value: number) => (total > 0 ? Math.round((value / total) * 1000) / 10 : 0);

  return (
    <div className={cn("flex min-w-0 flex-wrap items-center gap-x-6 gap-y-4", className)}>
      <div className="relative min-w-0 flex-1" style={{ height, minWidth: 180 }}>
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={slices}
              dataKey="value"
              nameKey="name"
              innerRadius="62%"
              outerRadius="92%"
              // 起点扳到 12 点、顺时针:与「从大到小顺读」的习惯一致
              startAngle={90}
              endAngle={-270}
              paddingAngle={1}
              stroke="none"
              isAnimationActive={!reducedMotion}
              animationDuration={550}
              animationEasing="ease-out"
            >
              {slices.map((slice, index) => (
                <Cell key={slice.key} fill={colors.series[index % colors.series.length]} />
              ))}
            </Pie>
            <Tooltip
              formatter={(value: number, sliceName: string) => [
                `${value} · ${percentOf(value)}%`,
                sliceName,
              ]}
              contentStyle={{
                backgroundColor: colors.surface,
                border: `1px solid ${colors.grid}`,
                borderRadius: 8,
                color: colors.ink,
                fontSize: 12,
              }}
            />
          </PieChart>
        </ResponsiveContainer>
        {/* 中心读数:绝对定位在环心,pointer-events-none 让鼠标穿透到扇区上 */}
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
          <span className={cn("font-display text-2xl tabular-nums", styles.total)}>{total}</span>
          <span className={cn("mt-0.5 text-xs", styles.label)}>{totalLabel}</span>
        </div>
      </div>

      {/* 图例:名称 + 数值 + 百分比。文字自带全部信息,不依赖读者分辨色块(§8.2) */}
      <ul className="flex min-w-0 shrink-0 basis-44 flex-col gap-2">
        {slices.map((slice, index) => (
          <li key={slice.key} className="flex items-center gap-2 text-sm">
            <span
              aria-hidden="true"
              className="size-2.5 shrink-0 rounded-sm"
              style={{ backgroundColor: colors.series[index % colors.series.length] }}
            />
            <span className={cn("min-w-0 flex-1 truncate", styles.name)}>{slice.name}</span>
            <span className={cn("shrink-0 tabular-nums", styles.label)}>
              {slice.value} · {percentOf(slice.value)}%
            </span>
          </li>
        ))}
      </ul>
    </div>
  );
}
