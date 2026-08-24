/**
 * TrendLineChart:趋势折线图(规范 §8.1「连续时间序列」)。
 * 系列色经 useChartColors 从令牌派生;多系列默认自动加虚线线型,保证颜色不是唯一区分手段(§8.2);
 * 图例可点切换系列显隐(§8.2 强制);reduced-motion 时关闭入场动画,数据立即可读。
 *
 * 同时承担 §8.1「时间序列但要找离群点」一档:传 threshold 画阈值参照线、传 anomalies 标出离群点。
 * 异常点用**三角形标记 + 数值文字**双载体,不靠变红 —— 只变颜色时色盲用户什么都看不到(§8.2 色非唯一)。
 *
 * 应包在 ChartContainer 内使用。
 */
import { useState } from "react";
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ReferenceDot,
  ReferenceLine,
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

/** 阈值参照线:异常点需要参照物,否则「高」是相对谁高说不清 */
export interface TrendThreshold {
  value: number;
  /** 用户向标签,如「阈值 5%」 */
  label: string;
}

/** 离群点标注:落在哪个横轴刻度、哪条系列、什么数值、怎么说 */
export interface TrendAnomaly {
  /** 横轴取值(与 data 里 xKey 的值一致) */
  x: string | number;
  /** 纵轴取值 */
  y: number;
  /** 点上方显示的用户向文字,如「11.2%」或「判题积压」 */
  label: string;
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
  /** 阈值参照线(可选):画一条冷红虚线 + 右端标签 */
  threshold?: TrendThreshold;
  /** 离群点(可选):三角标记 + 圆点 + 数值文字,不依赖颜色传达 */
  anomalies?: TrendAnomaly[];
}

/**
 * 离群点标记形状:向下指的三角形 + 实心圆点。
 * 三角形是形状载体(色盲/灰度下仍可辨),圆点锚定精确位置;
 * Recharts 的 shape 回调拿到的是已换算的像素坐标。
 */
function AnomalyMarker({ cx, cy, fill }: { cx?: number; cy?: number; fill: string }) {
  if (cx === undefined || cy === undefined) return null;
  return (
    <g>
      <path d={`M${cx} ${cy - 9} l6 -10 h-12 Z`} fill={fill} />
      <circle cx={cx} cy={cy} r={3.5} fill={fill} />
    </g>
  );
}

export function TrendLineChart({
  data,
  xKey,
  series,
  height = 280,
  context = "paper",
  threshold,
  anomalies,
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
        {/* 阈值参照线:冷红虚线(danger 语义),标签贴右端,不与数据线抢注意力 */}
        {threshold && (
          <ReferenceLine
            y={threshold.value}
            stroke={colors.danger}
            strokeDasharray="4 4"
            strokeOpacity={0.6}
            label={{
              value: threshold.label,
              position: "insideTopRight",
              fill: colors.danger,
              fontSize: 11,
            }}
          />
        )}

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

        {/* 离群点:形状(三角)+ 文字(数值)双载体,§8.2 不允许只靠颜色 */}
        {anomalies?.map((anomaly) => (
          <ReferenceDot
            key={`${anomaly.x}-${anomaly.y}-${anomaly.label}`}
            x={anomaly.x}
            y={anomaly.y}
            // 命中 series 的隐藏与否不影响标注:异常本身就是要一直看得见
            ifOverflow="extendDomain"
            shape={<AnomalyMarker fill={colors.danger} />}
            label={{
              value: anomaly.label,
              position: "top",
              offset: 22,
              fill: colors.danger,
              fontSize: 11,
            }}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  );
}
