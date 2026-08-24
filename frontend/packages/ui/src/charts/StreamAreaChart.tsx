/**
 * StreamAreaChart:实时流式面积图(规范 §8.1「连续时间序列 + 仍在变化」)。
 *
 * 与 TrendLineChart 的区别不在图形而在时间性:这里的数据在用户看着的时候还在变,
 * 所以三件事是强制的 ——
 * ① **当前值大字直读**:实时场景里「现在是多少」比「过去怎么走」更要紧,不能只让人从曲线末端估;
 * ② **必须能暂停**:流一直动的时候没法看清刚过去那一段,暂停后视图冻结在按下的那一刻;
 * ③ **reduced-motion 不做补间**:尊重系统偏好,数值按整帧更新而不是滑动过渡(§8.2)。
 *
 * 组件只负责呈现,数据由调用方持续追加(通常来自 WS 推送);窗口大小也由调用方决定 ——
 * 组件不替业务裁剪历史,否则「图上少了一段」的原因会散落在两个地方。
 * 应包在 ChartContainer 内使用(三态、数据表替代、读屏摘要由容器提供)。
 */
import { useState } from "react";
import { Pause, Play } from "lucide-react";
import { Area, AreaChart, CartesianGrid, ReferenceLine, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { Button } from "../components/Button";
import { useReducedMotion } from "../hooks/useReducedMotion";
import { cn } from "../lib/cn";
import { useChartColors, type ChartContext } from "./palette";

export interface StreamAreaChartProps {
  /** 时间序列点,按时间升序;调用方持续追加 */
  data: Array<Record<string, string | number>>;
  /** 横轴字段名(时间) */
  xKey: string;
  /** 数值字段名 */
  valueKey: string;
  /** 指标名(用户向),如「判题吞吐」 */
  name: string;
  /** 数值单位(用户向),如「条 / 秒」;不传则只显示数字 */
  unit?: string;
  /** 图表高度(px),默认 220 —— 比静态折线略矮,给上方的大字读数留位置 */
  height?: number;
  /** 配色语境:paper=宣纸光面(默认),dark=墨底沉浸舞台/深色面板 */
  context?: ChartContext;
  className?: string;
}

/** 两种语境的语义类:大字读数与辅助文字各走 ink / on-dark 系令牌 */
const STYLES: Record<ChartContext, { value: string; label: string }> = {
  paper: { value: "text-ink", label: "text-ink-sub" },
  dark: { value: "text-on-dark", label: "text-on-dark-sub" },
};

export function StreamAreaChart({
  data,
  xKey,
  valueKey,
  name,
  unit,
  height = 220,
  context = "paper",
  className,
}: StreamAreaChartProps) {
  const colors = useChartColors(context);
  const reducedMotion = useReducedMotion();
  const styles = STYLES[context];

  // 暂停:冻结的是**视图**,不是数据源。按下时把当前序列存成快照,恢复时丢掉快照回到实时。
  // 存快照而不是记长度:调用方可能同时从头部裁掉旧点,按长度切会切错位置。
  const [frozen, setFrozen] = useState<Array<Record<string, string | number>> | null>(null);
  const paused = frozen !== null;
  const view = frozen ?? data;

  // 大字读数取所在视图的最后一点:暂停时读数与图形必须是同一时刻,否则数字和曲线互相打架
  const latest = view.length > 0 ? view[view.length - 1][valueKey] : undefined;
  const gradientId = `stream-${valueKey}`;

  return (
    <div className={cn("flex min-w-0 flex-col gap-3", className)}>
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="min-w-0">
          <div className={cn("text-sm", styles.label)}>
            {name}
            {unit ? ` · ${unit}` : ""}
          </div>
          <div className={cn("font-display text-3xl tabular-nums", styles.value)}>
            {latest ?? "—"}
          </div>
        </div>
        <Button
          variant={context === "dark" ? "on-dark" : "outline"}
          size="sm"
          leftIcon={paused ? Play : Pause}
          onClick={() => setFrozen(paused ? null : data)}
        >
          {paused ? "继续" : "暂停"}
        </Button>
      </div>

      {/* 暂停态给出明确状态说明:否则「数字不动了」会被当成推送断了 */}
      {paused && (
        <p className={cn("text-xs", styles.label)} role="status">
          已暂停,画面停在按下暂停的那一刻;新数据仍在接收,点「继续」查看。
        </p>
      )}

      <ResponsiveContainer width="100%" height={height}>
        <AreaChart data={view} margin={{ top: 8, right: 12, bottom: 0, left: 0 }}>
          <defs>
            {/* 面积用同色渐变收到透明:面积表达「量」,渐变避免大色块压过曲线本身 */}
            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={colors.jade} stopOpacity={0.34} />
              <stop offset="100%" stopColor={colors.jade} stopOpacity={0} />
            </linearGradient>
          </defs>
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
          {/* 末点竖线标出「现在」:流式图里最新一点是读者的锚,没有它末端会看成随机截断 */}
          {view.length > 0 && (
            <ReferenceLine
              x={view[view.length - 1][xKey]}
              stroke={colors.jade}
              strokeDasharray="3 3"
              label={{ value: paused ? "暂停处" : "现在", position: "top", fill: colors.axis, fontSize: 11 }}
            />
          )}
          <Area
            type="monotone"
            dataKey={valueKey}
            name={name}
            stroke={colors.jade}
            strokeWidth={2}
            fill={`url(#${gradientId})`}
            dot={false}
            activeDot={{ r: 4 }}
            // reduced-motion:不做补间,数值按整帧更新(§8.2)
            isAnimationActive={!reducedMotion}
            animationDuration={200}
            animationEasing="linear"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
