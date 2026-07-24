// MatrixPatternRenderer 渲染投票、校验和状态矩阵。

import React from 'react';
import { clsx } from 'clsx';
import type { FrameFocus, MatrixPattern } from '../../types';
import { PatternHeader, PatternInsight, PatternLegend } from '../PatternChrome';
import { elementVisualClasses, selectableElementProps } from '../patternUtils';
import './MatrixPatternRenderer.css';

/**
 * 渲染投票、校验或状态矩阵,用文字与状态类共同表达结果。
 */
export function MatrixPatternRenderer({
  pattern,
  focus,
  selectedElementId,
  onSelectElement,
}: {
  pattern: MatrixPattern;
  focus?: FrameFocus;
  selectedElementId?: string;
  onSelectElement?: (elementId: string, elementType?: string) => void;
}): React.ReactElement {
  const statusCounts = pattern.data.cells.flat().reduce<Record<string, number>>((counts, cell) => {
    counts[cell.status] = (counts[cell.status] ?? 0) + 1;
    return counts;
  }, {});
  return (
    <section className="sim-pattern sim-pattern--matrix" aria-label={pattern.title}>
      <PatternHeader mode="matrix" title={pattern.title} meta={`通过 ${statusCounts.yes ?? 0} / 处理中 ${statusCounts.pending ?? 0} / 异常 ${statusCounts.fault ?? 0}`} />
      <PatternInsight items={[['拒绝', statusCounts.no ?? 0], ['等待', statusCounts.empty ?? 0]]} />
      <div className="sim-matrix-wrap">
        <table className="sim-matrix">
          <thead>
            <tr>
              <th scope="col">对象</th>
              {pattern.data.columns.map((column) => (
                <th scope="col" key={column}>
                  {column}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {pattern.data.rows.map((row, rowIndex) => (
              <tr key={row}>
                <th scope="row">{row}</th>
                {pattern.data.cells[rowIndex].map((cell, columnIndex) => {
                  const cellId = `${pattern.id}:${row}:${pattern.data.columns[columnIndex]}`;
                  return (
                    <td
                      className={clsx('sim-matrix__cell', `is-${cell.status}`, elementVisualClasses({ id: cellId, meta: cell.meta }, focus), selectedElementId === cellId && 'is-selected')}
                      key={`${row}-${columnIndex}`}
                      {...selectableElementProps(cellId, onSelectElement, 'cell')}
                    >
                      <span>{cell.label}</span>
                      <small>{matrixStatusLabel(cell.status)}</small>
                    </td>
                  );
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <PatternLegend items={[['通过', 'success'], ['处理中', 'accent'], ['等待', 'muted'], ['异常', 'danger']]} />
    </section>
  );
}

/**
 * 返回矩阵单元的可读状态标签。
 */
function matrixStatusLabel(status: MatrixPattern['data']['cells'][number][number]['status']): string {
  const labels = { empty: '未开始', pending: '进行中', yes: '通过', no: '拒绝', fault: '异常' };
  return labels[status];
}
