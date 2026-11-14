// TraceIdCell 显示审计记录的「追踪编号」(校管与平台两个审计页共用一份)。
//
// 编号只用于报障时复述:表格里展开成 36 位全量会吃掉四分之一表宽,读者依然读不出内容,
// 因此表格里只显示前 8 位,整串交给复制按钮与 title;详情抽屉里地方够,传 full 显示整串。

import { Copy } from 'lucide-react'
import { IconButton, toast } from '@chaimir/ui'
import { copyText } from '../../../utils/clipboard'

/** 表格里展示的编号位数:足以在报障时区分不同记录,又不挤占数据列。 */
const TRACE_ID_PREVIEW_LENGTH = 8

export interface TraceIdCellProps {
  traceId?: string
  /** 显示完整编号(详情抽屉) */
  full?: boolean
}

/**
 * TraceIdCell 显示编号(表格里前 8 位)+ 复制按钮;没有编号时写「没有记录」。
 * 复制失败由 copyText 统一给用户向提示与结构化日志,这里只负责成功反馈。
 */
export function TraceIdCell({ traceId, full = false }: TraceIdCellProps) {
  if (!traceId) return <span className="text-ink-sub">{full ? '没有记录' : '—'}</span>
  return (
    <span className="flex items-center gap-1">
      <span
        className={
          full
            ? 'break-all font-mono text-xs text-ink'
            : 'whitespace-nowrap font-mono text-xs text-ink-sub'
        }
        title={traceId}
      >
        {full ? traceId : traceId.slice(0, TRACE_ID_PREVIEW_LENGTH)}
      </span>
      <IconButton
        variant="ghost"
        size="sm"
        icon={Copy}
        aria-label={`复制追踪编号 ${traceId}`}
        onClick={() => {
          void copyText(traceId, {
            what: '追踪编号',
            operation: 'admin.audit.copyTraceId',
          }).then((ok) => {
            if (ok) toast.success('追踪编号已复制')
          })
        }}
      />
    </span>
  )
}
