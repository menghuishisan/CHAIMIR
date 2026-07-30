/**
 * CompareBarChart:维度对比柱状图(规范 §8.1 维度对比,>5 类别时替代饼图)。
 * 与 TrendLineChart 同风格:系列色经 useChartColors 从令牌派生,圆角柱顶;
 * 第 2 个系列起用斜纹 pattern 填充区分形状(色非唯一区分铁律,§8.2);
 * 图例可点切换系列显隐(§8.2 强制);reduced-motion 时关闭入场动画。
 * 应包在 ChartContainer 内使用。
 */
import { useId, useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { useReducedMotion } from "../hooks/useReducedMotion";
import { useChartColors, type ChartContext } from "./palette";

/** 柱状系列定义 */
export interface CompareSeries {
  /** 数据字段名 */
  key: string;
  /** 图例/提示中展示的系列名(用户向) */
  name: string;
}

export interface CompareBarChartProps {
  data: Array<Record<string, string | number>>;
  /** 横轴字段名(维度) */
  xKey: string;
  series: CompareSeries[];
  /** 图表高度(px),默认 280 */
  height?: number;
  /** 配色语境:paper=宣纸光面(默认),dark=墨底沉浸舞台/深色面板 */
  context?: ChartContext;
}

export function CompareBarChart({
  data,
  xKey,
  series,
  height = 280,
  context = "paper",
}: CompareBarChartProps) {
  const colors = useChartColors(context);
  const reducedMotion = useReducedMotion();
  // pattern id 用 useId 派生,防止同页多图 defs id 冲突(useId 含冒号,需清洗为合法 SVG id)
  const patternIdBase = useId().replace(/:/g, "");
  // 图例点击切换系列显隐(§8.2 强制):隐藏集合按 dataKey 记录,Bar 传 hide 保留图例项
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
      <BarChart data={data} margin={{ top: 8, right: 12, bottom: 0, left: 0 }}>
        {/* 多系列形状区分(色盲友好铁律,§8.2):第 2 个系列起以对应系列色的斜纹
            pattern 填充,灰度/色盲下仍可与第 1 系列的纯色区分 */}
        {series.length > 1 && (
          <defs>
            {series.slice(1).map((s, sliceIndex) => {
              const seriesColor = colors.series[(sliceIndex + 1) % colors.series.length];
              return (
                <pattern
                  key={s.key}
                  id={`${patternIdBase}-stripe-${sliceIndex + 1}`}
                  patternUnits="userSpaceOnUse"
                  width={6}
                  height={6}
                  patternTransform="rotate(45)"
                >
                  {/* 底色半透明 + 同色斜线:保持系列色相,叠加可辨纹理 */}
                  <rect width={6} height={6} fill={seriesColor} fillOpacity={0.35} />
                  <line x1={0} y1={0} x2={0} y2={6} stroke={seriesColor} strokeWidth={2.5} />
                </pattern>
              );
            })}
          </defs>
        )}
        {/* 网格线弱化:仅横线 + 稀疏虚线 */}
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
          cursor={{ fill: colors.grid, fillOpacity: 0.4 }}
          contentStyle={{
            backgroundColor: colors.surface,
            border: `1px solid ${colors.grid}`,
            borderRadius: 8,
            color: colors.ink,
            fontSize: 12,
          }}
          labelStyle={{ color: colors.ink }}
        />
        {/* 图例可点:cursor pointer 提示可交互,点击按 dataKey 切换对应系列 */}
        <Legend
          wrapperStyle={{ fontSize: 12, color: colors.axis, cursor: "pointer" }}
          iconSize={12}
          onClick={(item) => {
            if (typeof item.dataKey === "string") toggleSeries(item.dataKey);
          }}
        />
        {series.map((s, index) => (
          <Bar
            key={s.key}
            dataKey={s.key}
            name={s.name}
            hide={hiddenKeys.has(s.key)}
            // 第 1 系列纯色,第 2 起引用斜纹 pattern(形状区分,见上方 defs)
            fill={
              index === 0
                ? colors.series[0]
                : `url(#${patternIdBase}-stripe-${index})`
            }
            radius={[4, 4, 0, 0]}
            maxBarSize={40}
            isAnimationActive={!reducedMotion}
            // 值与令牌 --t-entrance(550ms,tokens/base.css)保持一致,改令牌时同步此处
            animationDuration={550}
            animationEasing="ease-out"
          />
        ))}
      </BarChart>
    </ResponsiveContainer>
  );
}
