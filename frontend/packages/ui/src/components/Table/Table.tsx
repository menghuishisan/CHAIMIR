// Table 组件：四端共享的数据表格，内置排序语义、加载态、空态和窄屏键值布局。

import React from 'react'
import { clsx } from 'clsx'
import { ArrowDown, ArrowUp, ArrowUpDown } from 'lucide-react'
import { Empty } from '../Empty'
import { Skeleton } from '../Skeleton'
import './Table.css'

export type TableSortDirection = 'asc' | 'desc'

export interface TableColumn<T extends Record<string, unknown>> {
  key: string
  title: React.ReactNode
  render?: (row: T, rowIndex: number) => React.ReactNode
  dataIndex?: keyof T
  sortable?: boolean
  sortDirection?: TableSortDirection
  priority?: 'primary' | 'secondary' | 'optional'
  align?: 'start' | 'center' | 'end'
}

export interface TableProps<T extends Record<string, unknown>> extends React.HTMLAttributes<HTMLDivElement> {
  columns: TableColumn<T>[]
  rows: T[]
  rowKey: keyof T | ((row: T, rowIndex: number) => string)
  loading?: boolean
  emptyTitle?: string
  emptyDescription?: string
  onSort?: (column: TableColumn<T>) => void
}

/**
 * Table 渲染同一份数据的桌面表格和移动键值卡片，避免小屏整页横向滚动。
 */
export function Table<T extends Record<string, unknown>>({
  columns,
  rows,
  rowKey,
  loading = false,
  emptyTitle = '暂无数据',
  emptyDescription = '当前没有可展示的记录',
  onSort,
  className,
  ...props
}: TableProps<T>): React.ReactElement {
  const visibleColumns = columns.filter((column) => column.priority !== 'optional')

  return (
    <div className={clsx('chaimir-table', className)} {...props}>
      <div className="chaimir-table__desktop" role="region" aria-label="数据表格">
        <table>
          <thead>
            <tr>
              {columns.map((column) => (
                <th key={column.key} scope="col" aria-sort={sortAria(column)} className={column.align ? `is-${column.align}` : undefined}>
                  {column.sortable ? (
                    <button type="button" className="chaimir-table__sort" onClick={() => onSort?.(column)}>
                      <span>{column.title}</span>
                      {sortIcon(column)}
                    </button>
                  ) : (
                    column.title
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>{loading ? skeletonRows(columns.length) : rows.map((row, rowIndex) => renderDesktopRow(row, rowIndex, columns, rowKey))}</tbody>
        </table>
      </div>
      <div className="chaimir-table__mobile">
        {loading
          ? Array.from({ length: 3 }, (_, index) => <Skeleton key={index} variant="block" height={96} />)
          : rows.map((row, rowIndex) => renderMobileRow(row, rowIndex, visibleColumns, rowKey))}
      </div>
      {!loading && rows.length === 0 && (
        <div className="chaimir-table__empty">
          <Empty title={emptyTitle} description={emptyDescription} />
        </div>
      )}
    </div>
  )
}

/**
 * renderDesktopRow 输出常规表格行，数字/成绩列可由调用方在 render 中自行格式化。
 */
function renderDesktopRow<T extends Record<string, unknown>>(
  row: T,
  rowIndex: number,
  columns: TableColumn<T>[],
  rowKey: TableProps<T>['rowKey']
): React.ReactElement {
  return (
    <tr key={resolveRowKey(row, rowIndex, rowKey)}>
      {columns.map((column) => (
        <td key={column.key} className={column.align ? `is-${column.align}` : undefined}>
          {cellValue(row, rowIndex, column)}
        </td>
      ))}
    </tr>
  )
}

/**
 * renderMobileRow 把一行数据转成键值卡片，小屏优先展示 primary/secondary 列。
 */
function renderMobileRow<T extends Record<string, unknown>>(
  row: T,
  rowIndex: number,
  columns: TableColumn<T>[],
  rowKey: TableProps<T>['rowKey']
): React.ReactElement {
  return (
    <article className="chaimir-table-card" key={resolveRowKey(row, rowIndex, rowKey)}>
      {columns.map((column) => (
        <div key={column.key}>
          <dt>{column.title}</dt>
          <dd>{cellValue(row, rowIndex, column)}</dd>
        </div>
      ))}
    </article>
  )
}

function cellValue<T extends Record<string, unknown>>(row: T, rowIndex: number, column: TableColumn<T>): React.ReactNode {
  if (column.render) {
    return column.render(row, rowIndex)
  }
  if (!column.dataIndex) {
    return ''
  }
  const value = row[column.dataIndex]
  return value === null || value === undefined ? '' : String(value)
}

function resolveRowKey<T extends Record<string, unknown>>(row: T, rowIndex: number, rowKey: TableProps<T>['rowKey']): string {
  if (typeof rowKey === 'function') {
    return rowKey(row, rowIndex)
  }
  return String(row[rowKey])
}

function skeletonRows(columnCount: number): React.ReactElement[] {
  return Array.from({ length: 5 }, (_, rowIndex) => (
    <tr key={rowIndex}>
      {Array.from({ length: columnCount }, (_unused, columnIndex) => (
        <td key={columnIndex}>
          <Skeleton variant="text" />
        </td>
      ))}
    </tr>
  ))
}

function sortAria<T extends Record<string, unknown>>(column: TableColumn<T>): 'ascending' | 'descending' | 'none' {
  if (column.sortDirection === 'asc') return 'ascending'
  if (column.sortDirection === 'desc') return 'descending'
  return 'none'
}

function sortIcon<T extends Record<string, unknown>>(column: TableColumn<T>): React.ReactElement {
  if (column.sortDirection === 'asc') return <ArrowUp size={14} aria-hidden="true" />
  if (column.sortDirection === 'desc') return <ArrowDown size={14} aria-hidden="true" />
  return <ArrowUpDown size={14} aria-hidden="true" />
}
