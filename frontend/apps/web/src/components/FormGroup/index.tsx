// FormGroup 长表单里的一个可折叠分组(设计 §7.6:登记运行时、工具、判题器的大抽屉「按分组折叠」)。
// 标题常驻,点标题展开或收起;有错误的分组标出「有需要修改的项」并保持展开,不让错误藏在收起的组里。
// 收起只是不显示,分组里的字段仍然参与提交。

import { useId, useState, type ReactNode } from 'react'
import { ChevronRight } from 'lucide-react'
import { cn, Icon } from '@chaimir/ui'

export interface FormGroupProps {
  title: string
  /** 标题右侧的一句摘要,如「已填 3 个端口」 */
  summary?: string
  /** 这一组里有校验错误:标出提示并强制展开 */
  invalid?: boolean
  /** 默认是否展开 */
  defaultOpen?: boolean
  children: ReactNode
}

/** FormGroup 可折叠的表单分组 */
export function FormGroup({
  title,
  summary,
  invalid = false,
  defaultOpen = false,
  children,
}: FormGroupProps) {
  const [open, setOpen] = useState(defaultOpen)
  const panelId = useId()
  const expanded = open || invalid
  return (
    <section className="border-b border-line last:border-b-0">
      <h3>
        <button
          type="button"
          aria-expanded={expanded}
          aria-controls={panelId}
          onClick={() => setOpen(!expanded)}
          className="flex w-full items-center gap-2 py-3 text-left outline-hidden focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
        >
          <Icon
            icon={ChevronRight}
            size="sm"
            className={cn(
              'shrink-0 text-ink-sub transition-transform duration-fast',
              expanded && 'rotate-90'
            )}
          />
          <span className="min-w-0 flex-1 text-base font-semibold text-ink">{title}</span>
          {invalid ? (
            <span className="shrink-0 text-xs font-medium text-danger">有需要修改的项</span>
          ) : summary ? (
            <span className="shrink-0 text-xs text-ink-sub">{summary}</span>
          ) : null}
        </button>
      </h3>
      <div id={panelId} hidden={!expanded} className="flex flex-col gap-3 pb-4">
        {children}
      </div>
    </section>
  )
}
