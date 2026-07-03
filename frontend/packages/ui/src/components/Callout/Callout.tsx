// Callout 组件：用于信息、警告、成功和危险提示，统一图标与用户向文案展示。

import React from 'react'
import { clsx } from 'clsx'
import { AlertTriangle, CheckCircle2, Info, ShieldAlert } from 'lucide-react'
import './Callout.css'

export type CalloutVariant = 'info' | 'warning' | 'success' | 'danger'

export interface CalloutProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: CalloutVariant
  title: string
  children?: React.ReactNode
}

const icons: Record<CalloutVariant, React.ReactElement> = {
  info: <Info size={18} />,
  warning: <AlertTriangle size={18} />,
  success: <CheckCircle2 size={18} />,
  danger: <ShieldAlert size={18} />,
}

export function Callout({ variant = 'info', title, children, className, ...props }: CalloutProps): React.ReactElement {
  return (
    <section className={clsx('chaimir-callout', `is-${variant}`, className)} role={variant === 'danger' ? 'alert' : 'status'} {...props}>
      <span className="chaimir-callout__icon" aria-hidden="true">
        {icons[variant]}
      </span>
      <div>
        <strong>{title}</strong>
        {children && <div className="chaimir-callout__body">{children}</div>}
      </div>
    </section>
  )
}
