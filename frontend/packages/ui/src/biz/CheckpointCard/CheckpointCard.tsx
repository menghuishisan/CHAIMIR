// CheckpointCard 业务组件：统一展示检查点判分结果和可恢复提示。

import React from 'react'
import { clsx } from 'clsx'
import { AlertTriangle, CheckCircle2, CircleDashed } from 'lucide-react'
import './CheckpointCard.css'

export type CheckpointStatus = 'pending' | 'passed' | 'failed'

export interface CheckpointCardProps extends React.HTMLAttributes<HTMLElement> {
  title: string
  status: CheckpointStatus
  score?: number
  description?: string
  action?: React.ReactNode
}

/**
 * CheckpointCard 展示检查点判定结果、得分和反馈，供实验与判题结果复用。
 */
export function CheckpointCard({ title, status, score, description, action, className, ...props }: CheckpointCardProps): React.ReactElement {
  return (
    <article className={clsx('chaimir-checkpoint-card', `is-${status}`, className)} {...props}>
      <span className="chaimir-checkpoint-card__icon" aria-hidden="true">
        {status === 'passed' ? <CheckCircle2 size={18} /> : status === 'failed' ? <AlertTriangle size={18} /> : <CircleDashed size={18} />}
      </span>
      <div>
        <strong>{title}</strong>
        {description && <p>{description}</p>}
      </div>
      {score !== undefined && <b>{score} 分</b>}
      {action && <div className="chaimir-checkpoint-card__action">{action}</div>}
    </article>
  )
}
