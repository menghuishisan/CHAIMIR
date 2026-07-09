// DescriptionList 组件：用于详情抽屉、审计信息、工作台状态和表格移动卡片中的键值信息。

import React from 'react'
import { clsx } from 'clsx'
import './DescriptionList.css'

export interface DescriptionItem {
  key: string
  label: React.ReactNode
  value: React.ReactNode
  tone?: 'default' | 'muted' | 'strong'
}

export interface DescriptionListProps extends React.HTMLAttributes<HTMLDListElement> {
  items: DescriptionItem[]
  columns?: 1 | 2 | 3
  density?: 'comfortable' | 'compact'
}

/**
 * DescriptionList 用 dl/dt/dd 渲染键值详情，适合审计、状态和移动端表格卡片。
 */
export function DescriptionList({
  items,
  columns = 1,
  density = 'comfortable',
  className,
  ...props
}: DescriptionListProps): React.ReactElement {
  return (
    <dl
      className={clsx(
        'chaimir-description-list',
        `chaimir-description-list--cols-${columns}`,
        `chaimir-description-list--${density}`,
        className
      )}
      {...props}
    >
      {items.map((item) => (
        <div className={clsx('chaimir-description-list__item', item.tone && `is-${item.tone}`)} key={item.key}>
          <dt>{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  )
}
