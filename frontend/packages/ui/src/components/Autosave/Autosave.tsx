// Autosave 组件：展示服务端草稿保存状态，多步骤流程以服务端为权威。

import React from 'react'
import { clsx } from 'clsx'
import { AlertTriangle, CheckCircle2, Loader2 } from 'lucide-react'
import './Autosave.css'

export type AutosaveStatus = 'idle' | 'saving' | 'saved' | 'error'

export interface AutosaveProps extends React.HTMLAttributes<HTMLDivElement> {
  status: AutosaveStatus
  savedAt?: string
  traceId?: string
}

/**
 * Autosave 展示服务端草稿保存状态，并在失败时保留可报障编号。
 */
export function Autosave({ status, savedAt, traceId, className, ...props }: AutosaveProps): React.ReactElement {
  const label = statusLabel(status, savedAt, traceId)
  return (
    <div className={clsx('chaimir-autosave', `is-${status}`, className)} role="status" aria-live="polite" {...props}>
      {status === 'saving' ? <Loader2 size={16} className="chaimir-autosave__spin" /> : status === 'error' ? <AlertTriangle size={16} /> : <CheckCircle2 size={16} />}
      <span>{label}</span>
    </div>
  )
}

/**
 * statusLabel 把保存状态转换为用户可理解的短文案。
 */
function statusLabel(status: AutosaveStatus, savedAt?: string, traceId?: string): string {
  if (status === 'saving') return '正在保存草稿'
  if (status === 'error') return traceId ? `草稿保存失败，请稍后重试。编号 ${traceId}` : '草稿保存失败，请稍后重试'
  if (status === 'saved') return savedAt ? `草稿已保存 ${savedAt}` : '草稿已保存'
  return '草稿会自动保存'
}
