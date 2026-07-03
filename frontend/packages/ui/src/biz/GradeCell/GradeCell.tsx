// GradeCell 业务组件：统一成绩展示、锁定状态和趋势文案。

import React from 'react'
import { clsx } from 'clsx'
import { Lock, Unlock } from 'lucide-react'
import './GradeCell.css'

export interface GradeCellProps extends React.HTMLAttributes<HTMLDivElement> {
  score: number | null
  locked?: boolean
  label?: string
}

export function GradeCell({ score, locked = false, label, className, ...props }: GradeCellProps): React.ReactElement {
  return (
    <div className={clsx('chaimir-grade-cell', locked && 'is-locked', className)} {...props}>
      <strong>{score === null ? '未评分' : score.toFixed(1)}</strong>
      <span>
        {locked ? <Lock size={13} /> : <Unlock size={13} />}
        {label ?? (locked ? '已锁定' : '可更新')}
      </span>
    </div>
  )
}
