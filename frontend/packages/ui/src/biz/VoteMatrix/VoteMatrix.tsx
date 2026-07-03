// VoteMatrix 业务组件：统一展示 PBFT、Raft、检查项等跨端复用矩阵。

import React from 'react'
import { CheckCircle2, Clock3, HelpCircle, XCircle } from 'lucide-react'
import { clsx } from 'clsx'
import './VoteMatrix.css'

export type VoteMatrixCellStatus = 'yes' | 'no' | 'pending' | 'fault' | 'neutral'

export interface VoteMatrixCell {
  label: string
  status: VoteMatrixCellStatus
  detail?: string
}

export interface VoteMatrixProps extends React.HTMLAttributes<HTMLElement> {
  title: string
  summary?: string
  rows: string[]
  columns: string[]
  cells: VoteMatrixCell[][]
}

const statusMeta: Record<VoteMatrixCellStatus, { label: string; icon: React.ReactElement }> = {
  yes: { label: '通过', icon: <CheckCircle2 size={15} /> },
  no: { label: '未通过', icon: <XCircle size={15} /> },
  pending: { label: '等待', icon: <Clock3 size={15} /> },
  fault: { label: '异常', icon: <XCircle size={15} /> },
  neutral: { label: '参考', icon: <HelpCircle size={15} /> },
}

/**
 * VoteMatrix 使用原生表格语义承载矩阵，读屏和窄屏都能读取每个交叉点。
 */
export function VoteMatrix({ title, summary, rows, columns, cells, className, ...props }: VoteMatrixProps): React.ReactElement {
  return (
    <section className={clsx('chaimir-vote-matrix', className)} aria-label={title} {...props}>
      <header className="chaimir-vote-matrix__header">
        <h3>{title}</h3>
        {summary && <p>{summary}</p>}
      </header>
      <div className="chaimir-vote-matrix__wrap">
        <table>
          <caption>{summary ?? title}</caption>
          <thead>
            <tr>
              <th scope="col">对象</th>
              {columns.map((column) => <th key={column} scope="col">{column}</th>)}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, rowIndex) => (
              <tr key={row}>
                <th scope="row">{row}</th>
                {columns.map((column, columnIndex) => {
                  const cell = cells[rowIndex]?.[columnIndex] ?? { label: '暂无数据', status: 'neutral' as const }
                  const meta = statusMeta[cell.status]
                  return (
                    <td key={column} className={`is-${cell.status}`}>
                      <span>
                        {React.cloneElement(meta.icon, { 'aria-hidden': true })}
                        <strong>{cell.label}</strong>
                      </span>
                      <small>{cell.detail ?? meta.label}</small>
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}
