// TraceIdCell 渲染审计表的「操作编号」单元格(校管与平台两个审计页共用一份)。
//
// 编号只用于报障时复述(规范 §6.7):展开成 36 位全量会吃掉四分之一表宽,读者依然读不出内容,
// 因此只显示前 8 位,整串交给复制按钮与 title。两个审计页共用本组件,避免同一单元格两处实现。

import { Copy } from 'lucide-react'
import { IconButton, toast } from '@chaimir/ui'
import { copyText } from '../../utils/clipboard'

/** 表格里展示的编号位数:足以在报障时区分不同记录,又不挤占数据列。 */
const TRACE_ID_PREVIEW_LENGTH = 8

export interface TraceIdCellProps {
  traceId?: string
}

/**
 * TraceIdCell 显示编号前 8 位 + 复制按钮;没有编号时显示占位符。
 * 复制失败由 copyText 统一给用户向提示与结构化日志,这里只负责成功反馈。
 */
export function TraceIdCell({ traceId }: TraceIdCellProps) {
  if (!traceId) return <span className="text-ink-sub">—</span>
  return (
    <span className="flex items-center gap-1">
      <span className="whitespace-nowrap font-mono text-xs text-ink-sub" title={traceId}>
        {traceId.slice(0, TRACE_ID_PREVIEW_LENGTH)}
      </span>
      <IconButton
        variant="ghost"
        size="sm"
        icon={Copy}
        aria-label={`复制操作编号 ${traceId}`}
        onClick={() => {
          void copyText(traceId, {
            what: '操作编号',
            operation: 'admin.audit.copyTraceId',
          }).then((ok) => {
            if (ok) toast.success('操作编号已复制')
          })
        }}
      />
    </span>
  )
}
