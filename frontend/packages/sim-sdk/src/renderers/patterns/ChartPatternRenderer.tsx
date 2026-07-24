// ChartPatternRenderer 渲染数值趋势和等价数据表。

import React from 'react';
import { clsx } from 'clsx';
import type { ChartPattern, ChartSeries } from '../../types';
import { PatternHeader, PatternInsight } from '../PatternChrome';
import { selectableElementProps } from '../patternUtils';
import './ChartPatternRenderer.css';

/**
 * 渲染轻量数值趋势,用于算力、延迟、难度、余额等随步进变化的数据。
 */
export function ChartPatternRenderer({
  pattern,
  selectedElementId,
  onSelectElement,
}: {
  pattern: ChartPattern;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const maxY = Math.max(1, ...pattern.data.series.flatMap((series) => series.points.map((point) => point.y)));
  const minX = Math.min(0, ...pattern.data.series.flatMap((series) => series.points.map((point) => point.x)));
  const maxX = Math.max(1, ...pattern.data.series.flatMap((series) => series.points.map((point) => point.x)));
  const ticks = chartTicks(maxY);
  const latest = pattern.data.series.map((series) => series.points[series.points.length - 1]?.y ?? 0);
  return (
    <section className="sim-pattern sim-pattern--chart" aria-label={pattern.title}>
      <PatternHeader mode="chart" title={pattern.title} meta={`最大值 ${maxY}${pattern.data.unit}`} />
      <PatternInsight items={[['序列', pattern.data.series.length], ['最新合计', latest.reduce((sum, value) => sum + value, 0)]]} />
      <div className="sim-chart">
        <svg className="sim-chart__plot" viewBox="0 0 100 60" role="img" aria-label={`${pattern.title}趋势图`}>
          <line className="sim-chart__axis" x1="8" y1="52" x2="96" y2="52" />
          <line className="sim-chart__axis" x1="8" y1="4" x2="8" y2="52" />
          {ticks.map((tick) => (
            <g key={tick}>
              <line className="sim-chart__grid" x1="8" x2="96" y1={chartY(tick, maxY)} y2={chartY(tick, maxY)} />
              <text className="sim-chart__tick" x="2" y={chartY(tick, maxY) + 2}>{tick}</text>
            </g>
          ))}
          <line className="sim-chart__current-marker" x1={chartPoint({ x: maxX, y: 0 }, minX, maxX, maxY).x} y1="4" x2={chartPoint({ x: maxX, y: 0 }, minX, maxX, maxY).x} y2="52" />
          {pattern.data.series.map((series, index) => {
            const visiblePoints = series.points.slice(-16);
            return (
              <g className={`sim-chart__series-line is-${index % 4}`} key={series.label}>
                <polyline points={chartPolyline(series, minX, maxX, maxY)} />
                {visiblePoints.map((point, pointIndex) => {
                  const pointId = `${pattern.id}:${series.label}:${point.x}`;
                  const plotted = chartPoint(point, minX, maxX, maxY);
                  return (
                    <circle
                      className={clsx(pointIndex === visiblePoints.length - 1 && 'is-latest', selectedElementId === pointId && 'is-selected')}
                      cx={plotted.x}
                      cy={plotted.y}
                      key={`${series.label}-${point.x}`}
                      r="1.7"
                      {...selectableElementProps(pointId, onSelectElement, 'point')}
                    />
                  );
                })}
              </g>
            );
          })}
        </svg>
        <div className="sim-chart__legend">
          {pattern.data.series.map((series, index) => (
            <span className={`is-${index % 4}`} key={series.label}>
              {series.label}
            </span>
          ))}
        </div>
        <table className="sim-chart__table">
          <caption>趋势数据</caption>
          <thead>
            <tr>
              <th scope="col">系列</th>
              <th scope="col">最新时间</th>
              <th scope="col">最新数值</th>
            </tr>
          </thead>
          <tbody>
            {pattern.data.series.map((series) => {
              const lastPoint = series.points[series.points.length - 1];
              return (
                <tr key={series.label}>
                  <th scope="row">{series.label}</th>
                  <td>{lastPoint?.x ?? 0}</td>
                  <td>{lastPoint ? `${lastPoint.y}${pattern.data.unit}` : `0${pattern.data.unit}`}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}

/**
 * 将图表序列转换为 SVG polyline 点串。
 */
function chartPolyline(series: ChartSeries, minX: number, maxX: number, maxY: number): string {
  return series.points
    .slice(-16)
    .map((point) => {
      const plotted = chartPoint(point, minX, maxX, maxY);
      return `${plotted.x},${plotted.y}`;
    })
    .join(' ');
}

/**
 * 将数据点映射到图表逻辑坐标。
 */
function chartPoint(point: { x: number; y: number }, minX: number, maxX: number, maxY: number): { x: number; y: number } {
  const x = 8 + ((point.x - minX) / Math.max(1, maxX - minX)) * 88;
  const y = chartY(point.y, maxY);
  return { x, y };
}

/**
 * chartTicks 根据真实最大值生成刻度,避免所有趋势图都显示固定 100 的误导性标尺。
 */
function chartTicks(maxY: number): number[] {
  const ceiling = Math.max(1, Math.ceil(maxY));
  return [0.25, 0.5, 0.75, 1].map((ratio) => Math.round(ceiling * ratio));
}

/**
 * chartY 将数值映射到图表坐标,所有趋势线和网格使用同一标尺。
 */
function chartY(value: number, maxY: number): number {
  return 52 - (value / Math.max(1, maxY)) * 46;
}
