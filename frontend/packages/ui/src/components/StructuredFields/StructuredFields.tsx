// StructuredFields 提供已建模键值、时间线和代码内容的可读呈现原语。

import React, { useEffect, useState } from 'react'
import { Check, Copy } from 'lucide-react'
import { Button } from '../Button'
import './StructuredFields.css'

export interface KeyValueItem {
  key: string
  label: React.ReactNode
  value: React.ReactNode
  description?: React.ReactNode
}

export interface KeyValueTableProps {
  items: KeyValueItem[]
  ariaLabel: string
  emptyLabel?: string
  className?: string
}

export interface TimelineItem {
  id: string
  title: React.ReactNode
  description?: React.ReactNode
  time?: React.ReactNode
  meta?: React.ReactNode
  tone?: 'neutral' | 'success' | 'warning' | 'danger' | 'info'
}

export interface TimelineProps {
  items: TimelineItem[]
  ariaLabel: string
  emptyLabel?: string
  className?: string
}

export interface CodeBlockProps {
  code: string
  ariaLabel: string
  language?: string
  copyLabel?: string
  className?: string
}

/** KeyValueTable 以调用方显式建模的行呈现同构键值，不接收自由对象。 */
export function KeyValueTable({ items, ariaLabel, emptyLabel = '暂无详细信息', className }: KeyValueTableProps): React.ReactElement {
  if (items.length === 0) return <p className="chaimir-structured__empty">{emptyLabel}</p>

  return (
    <div className={className} role="region" aria-label={ariaLabel}>
      <table className="chaimir-key-value-table">
        <tbody>
          {items.map((item) => (
            <tr key={item.key}>
              <th scope="row">{item.label}</th>
              <td>
                <span>{item.value}</span>
                {item.description && <small>{item.description}</small>}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

/** Timeline 以有序列表呈现调用方已映射的业务事件。 */
export function Timeline({ items, ariaLabel, emptyLabel = '暂无过程记录', className }: TimelineProps): React.ReactElement {
  if (items.length === 0) return <p className="chaimir-structured__empty">{emptyLabel}</p>

  return (
    <ol className={`chaimir-timeline${className ? ` ${className}` : ''}`} aria-label={ariaLabel}>
      {items.map((item) => (
        <li key={item.id} className={`is-${item.tone ?? 'neutral'}`}>
          <span className="chaimir-timeline__marker" aria-hidden="true" />
          <div>
            <div className="chaimir-timeline__heading">
              <strong>{item.title}</strong>
              {item.time && <time>{item.time}</time>}
            </div>
            {item.description && <p>{item.description}</p>}
            {item.meta && <small>{item.meta}</small>}
          </div>
        </li>
      ))}
    </ol>
  )
}

/** CodeBlock 展示源码、命令或终端输出，并显式处理剪贴板失败。 */
export function CodeBlock({ code, ariaLabel, language, copyLabel = '复制内容', className }: CodeBlockProps): React.ReactElement {
  const [copyState, setCopyState] = useState<'idle' | 'copied' | 'error'>('idle')

  useEffect(() => {
    if (copyState === 'idle') return
    const timer = window.setTimeout(() => setCopyState('idle'), 2000)
    return () => window.clearTimeout(timer)
  }, [copyState])

  /** copyCode 将已展示内容写入剪贴板，失败时保留用户可见反馈。 */
  const copyCode = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(code)
      setCopyState('copied')
    } catch {
      setCopyState('error')
    }
  }

  return (
    <figure className={`chaimir-code-block${className ? ` ${className}` : ''}`} aria-label={ariaLabel}>
      <figcaption>
        <span>{language || '文本'}</span>
        <Button
          type="button"
          variant="on-dark"
          size="sm"
          icon={copyState === 'copied' ? <Check size={14} /> : <Copy size={14} />}
          onClick={() => void copyCode()}
        >
          {copyState === 'copied' ? '已复制' : copyLabel}
        </Button>
      </figcaption>
      <pre><code>{code}</code></pre>
      {copyState === 'error' && <p role="alert">复制失败，请选中内容后手动复制。</p>}
    </figure>
  )
}
