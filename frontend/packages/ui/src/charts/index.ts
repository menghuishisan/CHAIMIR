/**
 * charts 出口:图表层(规范 §8)统一导出。
 * 使用约定:所有图表包在 ChartContainer 内(三态 + 数据表替代 + 读屏摘要),
 * 颜色一律经 useChartColors 从令牌派生。
 */
export { useChartColors, type ChartColors, type ChartContext } from "./palette";
export { ChartContainer, type ChartContainerProps, type ChartDataTable } from "./ChartContainer";
export { TrendLineChart, type TrendLineChartProps, type TrendSeries } from "./TrendLineChart";
export { CompareBarChart, type CompareBarChartProps, type CompareSeries } from "./CompareBarChart";
