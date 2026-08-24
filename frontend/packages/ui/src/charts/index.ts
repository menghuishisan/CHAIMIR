/**
 * charts 出口:图表层(规范 §8)统一导出。
 * 使用约定:所有图表包在 ChartContainer 内(三态 + 数据表替代 + 读屏摘要),
 * 颜色一律经 useChartColors 从令牌派生。
 * 选型判据是**数据形状**而非页面类型(§8.1),同一页不同位置可以是不同图形。
 */
export { useChartColors, type ChartColors, type ChartContext } from "./palette";
export { ChartContainer, type ChartContainerProps, type ChartDataTable } from "./ChartContainer";
export {
  TrendLineChart,
  type TrendLineChartProps,
  type TrendSeries,
  type TrendThreshold,
  type TrendAnomaly,
} from "./TrendLineChart";
export { CompareBarChart, type CompareBarChartProps, type CompareSeries } from "./CompareBarChart";
export { StreamAreaChart, type StreamAreaChartProps } from "./StreamAreaChart";
export { ShareDonutChart, type ShareDonutChartProps, type ShareSlice } from "./ShareDonutChart";
export { DensityMatrix, type DensityMatrixProps } from "./DensityMatrix";
export {
  TopologyGraph,
  type TopologyGraphProps,
  type TopologyNode,
  type TopologyEdge,
  type TopologyTone,
} from "./TopologyGraph";
