// SandboxStatus 业务组件：统一展示沙箱/虚拟机准备、运行、失败和释放状态。

import React from 'react'
import { clsx } from 'clsx'
import { AlertTriangle, CheckCircle2, Clock3, Hammer, PackageOpen, PlayCircle, ServerCrash } from 'lucide-react'
import './SandboxStatus.css'

export type SandboxStatusKind = 'ready' | 'compiling' | 'mining' | 'sealing' | 'failed' | 'released'

export interface SandboxStatusProps extends React.HTMLAttributes<HTMLDivElement> {
  status: SandboxStatusKind
  detail?: string
  traceId?: string
  onOpenReason?: () => void
}

const statusMeta: Record<SandboxStatusKind, { label: string; icon: React.ReactElement }> = {
  ready: { label: '环境已就绪', icon: <CheckCircle2 size={16} /> },
  compiling: { label: '正在编译', icon: <Hammer size={16} /> },
  mining: { label: '正在出块', icon: <PlayCircle size={16} /> },
  sealing: { label: '正在封块', icon: <PackageOpen size={16} /> },
  failed: { label: '环境准备失败', icon: <ServerCrash size={16} /> },
  released: { label: '环境已释放', icon: <Clock3 size={16} /> },
}

export function SandboxStatus({ status, detail, traceId, onOpenReason, className, ...props }: SandboxStatusProps): React.ReactElement {
  const meta = statusMeta[status]
  return (
    <div className={clsx('chaimir-sandbox-status', `is-${status}`, className)} {...props}>
      <span className="chaimir-sandbox-status__icon" aria-hidden="true">
        {meta.icon}
      </span>
      <span>
        <strong>{meta.label}</strong>
        {detail && <small>{detail}</small>}
        {traceId && <small>如需帮助,请提供编号 {traceId}</small>}
      </span>
      {status === 'failed' && onOpenReason && (
        <button type="button" onClick={onOpenReason}>
          <AlertTriangle size={14} />
          查看原因
        </button>
      )}
    </div>
  )
}
