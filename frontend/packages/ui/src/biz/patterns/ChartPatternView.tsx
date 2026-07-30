/**
 * ChartPatternView:趋势视图(chart 模式)。
 * 把协议的列式 `ChartSeries.points[{x,y}]` 合并成 Recharts 需要的行式数据(按 x 对齐),
 * 并统一包进 ChartContainer(context="dark")以获得读屏摘要与数据表替代(§8.2 强制)。
 * 标题由舞台统一渲染,故此处不再传 title,避免同一视图出现两级标题。
 */
import { ChartContainer, type ChartDataTable } from "../../charts/ChartContainer";
import { TrendLineChart, type TrendSeries } from "../../charts/TrendLineChart";
import type { PatternViewProps } from "./types";
import type { ChartPattern } from "@chaimir/sim-sdk";

/** x 轴字段名:协议里 x 是数值刻度(通常为 tick),统一用固定字段名承载 */
const X_KEY = "刻度";

/**
 * toRows:列式序列 → 行式数据。
 * 取所有序列 x 的并集并升序排列,每行是该刻度上各序列的值;某序列在该刻度无点则该列缺省,
 * Recharts 对缺省值断线,如实反映「该刻度无观测」而不是伪造 0。
 */
function toRows(series: ChartPattern["data"]["series"]): Array<Record<string, string | number>> {
  const xs = [...new Set(series.flatMap((item) => item.points.map((point) => point.x)))].sort(
    (left, right) => left - right,
  );
  return xs.map((x) => {
    const row: Record<string, string | number> = { [X_KEY]: x };
    for (const item of series) {
      const point = item.points.find((candidate) => candidate.x === x);
      if (point) row[item.label] = point.y;
    }
    return row;
  });
}

/** ariaSummary:用起止刻度与各序列末值给出关键洞察,读屏用户无需看图即可获知结论 */
function summarize(pattern: ChartPattern): string {
  const { series, unit } = pattern.data;
  const parts = series.map((item) => {
    const last = item.points[item.points.length - 1];
    return last ? `${item.label} 当前 ${last.y}${unit}` : `${item.label} 暂无数据`;
  });
  return `${pattern.title}:${parts.join(",")}`;
}

export function ChartPatternView({ pattern, density }: PatternViewProps<ChartPattern>) {
  const rows = toRows(pattern.data.series);
  const trendSeries: TrendSeries[] = pattern.data.series.map((item) => ({
    key: item.label,
    name: `${item.label}(${pattern.data.unit})`,
  }));
  const dataTable: ChartDataTable = {
    columns: [X_KEY, ...pattern.data.series.map((item) => `${item.label}(${pattern.data.unit})`)],
    rows: rows.map((row) => [
      row[X_KEY],
      ...pattern.data.series.map((item) => row[item.label] ?? "无数据"),
    ]),
  };
  // 窄侧栏给更矮的画布,宽舞台给标准高度
  const height = density === "panel" ? 160 : 280;

  return (
    <ChartContainer
      context="dark"
      height={height}
      isEmpty={rows.length === 0}
      emptyHint="仿真推进后这里会出现趋势曲线。"
      ariaSummary={summarize(pattern)}
      dataTable={dataTable}
    >
      <TrendLineChart data={rows} xKey={X_KEY} series={trendSeries} height={height} context="dark" />
    </ChartContainer>
  );
}
