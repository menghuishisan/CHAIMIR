// PageScaffold 组件：提供日常页面的可组合头部、主体和侧栏原语，不绑定四端业务布局。

import React from 'react'
import { clsx } from 'clsx'
import './PageScaffold.css'

export interface PageScaffoldProps extends React.HTMLAttributes<HTMLElement> {
  children: React.ReactNode
  as?: 'main' | 'div'
}

export interface PageHeaderProps extends Omit<React.HTMLAttributes<HTMLElement>, 'title'> {
  eyebrow?: React.ReactNode
  title: React.ReactNode
  description?: React.ReactNode
  icon?: React.ReactNode
  actions?: React.ReactNode
}

export interface PageBodyProps extends React.HTMLAttributes<HTMLDivElement> {
  children: React.ReactNode
  rail?: React.ReactNode
  density?: 'comfortable' | 'compact'
}

export interface PageSectionProps extends Omit<React.HTMLAttributes<HTMLElement>, 'title'> {
  title?: React.ReactNode
  description?: React.ReactNode
  actions?: React.ReactNode
  children: React.ReactNode
}

/**
 * PageScaffold 为四端日常页面提供统一外层间距和背景，不决定页面内容结构。
 */
export function PageScaffold({ children, as: Component = 'main', className, ...props }: PageScaffoldProps): React.ReactElement {
  return (
    <Component className={clsx('chaimir-page-scaffold', className)} {...props}>
      {children}
    </Component>
  )
}

/**
 * PageHeader 呈现页面定位信息和操作区，避免业务页重复实现标题层级。
 */
export function PageHeader({ eyebrow, title, description, icon, actions, className, ...props }: PageHeaderProps): React.ReactElement {
  return (
    <header className={clsx('chaimir-page-header', className)} {...props}>
      <div className="chaimir-page-header__main">
        {icon && <span className="chaimir-page-header__icon" aria-hidden="true">{icon}</span>}
        <div>
          {eyebrow && <p className="chaimir-page-header__eyebrow">{eyebrow}</p>}
          <h1>{title}</h1>
          {description && <p className="chaimir-page-header__description">{description}</p>}
        </div>
      </div>
      {actions && <div className="chaimir-page-header__actions">{actions}</div>}
    </header>
  )
}

/**
 * PageBody 提供主内容与可选侧栏的响应式栅格，四端可自行决定插入哪些业务块。
 */
export function PageBody({ children, rail, density = 'comfortable', className, ...props }: PageBodyProps): React.ReactElement {
  return (
    <div className={clsx('chaimir-page-body', rail && 'has-rail', `is-${density}`, className)} {...props}>
      <div className="chaimir-page-body__main">{children}</div>
      {rail && <aside className="chaimir-page-body__rail">{rail}</aside>}
    </div>
  )
}

/**
 * PageSection 封装普通页面区块标题、描述和动作槽，保持可组合而非卡片套卡片。
 */
export function PageSection({ title, description, actions, children, className, ...props }: PageSectionProps): React.ReactElement {
  return (
    <section className={clsx('chaimir-page-section', className)} {...props}>
      {(title || description || actions) && (
        <header className="chaimir-page-section__header">
          <div>
            {title && <h2>{title}</h2>}
            {description && <p>{description}</p>}
          </div>
          {actions && <div className="chaimir-page-section__actions">{actions}</div>}
        </header>
      )}
      <div className="chaimir-page-section__content">{children}</div>
    </section>
  )
}
