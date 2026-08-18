/**
 * TrendLineChart:趋势折线图(规范 §8.1 成绩/活跃趋势)。
 * 系列色经 useChartColors 从令牌派生;多系列默认自动加虚线线型,保证颜色不是唯一区分手段(§8.2);
 * 图例可点切换系列显隐(§8.2 强制);reduced-motion 时关闭入场动画,数据立即可读。
 * 应包在 ChartContainer 内使用。
 */
import { useState } from "react";
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { useReducedMotion } from "../hooks/useReducedMotion";
import { InteractiveLegend } from "./InteractiveLegend";
import { useChartColors, type ChartContext } from "./palette";

/** 折线系列定义 */
export interface TrendSeries {
  /** 数据字段名 */
  key: string;
  /** 图例/提示中展示的系列名(用户向) */
  name: string;
  /**
   * 虚线线型(6 4)。不传时第 2 条起默认虚线(色非唯一区分铁律,§8.2);
   * 显式传 false 可关闭默认虚线。
   */
  dash?: boolean;
}

export interface TrendLineChartProps {
  data: Array<Record<string, string | number>>;
  /** 横轴字段名 */
  xKey: string;
  series: TrendSeries[];
  /** 图表高度(px),默认 280 */
  height?: number;
  /** 配色语境:paper=宣纸光面(默认),dark=墨底沉浸舞台/深色面板 */
  context?: ChartContext;
}

export function TrendLineChart({
  data,
  xKey,
  series,
  height = 280,
  context = "paper",
}: TrendLineChartProps) {
  const colors = useChartColors(context);
  const reducedMotion = useReducedMotion();
  // 图例点击切换系列显隐(§8.2 强制):隐藏集合按 dataKey 记录,Line 传 hide 保留图例项
  const [hiddenKeys, setHiddenKeys] = useState<Set<string>>(new Set());
  const toggleSeries = (key: string) => {
    setHiddenKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  };

  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={data} margin={{ top: 8, right: 12, bottom: 0, left: 0 }}>
        {/* 网格线弱化:仅横线 + 稀疏虚线,不与数据线争夺注意力 */}
        <CartesianGrid stroke={colors.grid} strokeDasharray="3 6" vertical={false} />
        <XAxis
          dataKey={xKey}
          stroke={colors.axis}
          tick={{ fill: colors.axis, fontSize: 12 }}
          tickLine={false}
          axisLine={{ stroke: colors.grid }}
        />
        <YAxis
          stroke={colors.axis}
          tick={{ fill: colors.axis, fontSize: 12 }}
          tickLine={false}
          axisLine={false}
          width={44}
        />
        <Tooltip
          cursor={{ stroke: colors.grid }}
          contentStyle={{
            backgroundColor: colors.surface,
            border: `1px solid ${colors.grid}`,
            borderRadius: 8,
            color: colors.ink,
            fontSize: 12,
          }}
          labelStyle={{ color: colors.ink }}
        />
        <Legend
          content={
            <InteractiveLegend
              context={context}
              hiddenKeys={hiddenKeys}
              onToggle={toggleSeries}
              series={series}
            />
          }
        />
        {series.map((s, index) => (
          <Line
            key={s.key}
            type="monotone"
            dataKey={s.key}
            name={s.name}
            hide={hiddenKeys.has(s.key)}
            stroke={colors.series[index % colors.series.length]}
            strokeWidth={2}
            // 未显式指定 dash 时,第 2 条起默认虚线:色盲/灰度下仍可区分系列
            strokeDasharray={(s.dash ?? index >= 1) ? "6 4" : undefined}
            dot={false}
            activeDot={{ r: 4 }}
            isAnimationActive={!reducedMotion}
            // 值与令牌 --t-entrance(550ms,tokens/base.css)保持一致,改令牌时同步此处
            animationDuration={550}
            animationEasing="ease-out"
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  );
}
