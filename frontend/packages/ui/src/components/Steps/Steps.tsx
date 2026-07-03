// Steps 组件：用于实验编排、导入预览、入驻申请等多步骤流程的进度展示。

import React from 'react'
import { clsx } from 'clsx'
import { Check } from 'lucide-react'
import './Steps.css'

export type StepStatus = 'pending' | 'active' | 'done' | 'error'

export interface StepItem {
  id: string
  title: string
  description?: string
  status: StepStatus
}

export interface StepsProps extends React.HTMLAttributes<HTMLOListElement> {
  steps: StepItem[]
}

export function Steps({ steps, className, ...props }: StepsProps): React.ReactElement {
  return (
    <ol className={clsx('chaimir-steps', className)} {...props}>
      {steps.map((step, index) => (
        <li className={`is-${step.status}`} key={step.id} aria-current={step.status === 'active' ? 'step' : undefined}>
          <span className="chaimir-steps__marker" aria-hidden="true">
            {step.status === 'done' ? <Check size={14} /> : index + 1}
          </span>
          <span className="chaimir-steps__content">
            <strong>{step.title}</strong>
            {step.description && <small>{step.description}</small>}
          </span>
        </li>
      ))}
    </ol>
  )
}
