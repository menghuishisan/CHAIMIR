// Breadcrumb 组件：面包屑导航

import React from 'react'
import { ChevronRight } from 'lucide-react'
import { clsx } from 'clsx'
import { triggerHaptic } from '../../utils/haptics'
import './Breadcrumb.css'

export interface BreadcrumbItem {
  key: string
  label: React.ReactNode
  href?: string
  onClick?: () => void
}

export interface BreadcrumbProps {
  /** 面包屑项 */
  items: BreadcrumbItem[]
  /** 分隔符 */
  separator?: React.ReactNode
  /** 自定义类名 */
  className?: string
}

export const Breadcrumb: React.FC<BreadcrumbProps> = ({
  items,
  separator = <ChevronRight size={14} />,
  className,
}) => {
  const classes = clsx('chaimir-breadcrumb', className)

  return (
    <nav className={classes} aria-label="面包屑导航">
      <ol className="chaimir-breadcrumb__list">
        {items.map((item, index) => {
          const isLast = index === items.length - 1

          return (
            <li key={item.key} className="chaimir-breadcrumb__item">
              {isLast ? (
                <span className="chaimir-breadcrumb__current" aria-current="page">
                  {item.label}
                </span>
              ) : item.href ? (
                <a href={item.href} className="chaimir-breadcrumb__link">
                  {item.label}
                </a>
              ) : (
                <button
                  type="button"
                  className="chaimir-breadcrumb__link"
                  onClick={() => {
                    triggerHaptic(10)
                    if (item.onClick) item.onClick()
                  }}
                >
                  {item.label}
                </button>
              )}
              {!isLast && (
                <span className="chaimir-breadcrumb__separator" aria-hidden="true">
                  {separator}
                </span>
              )}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}

Breadcrumb.displayName = 'Breadcrumb'
